package data

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/auditx"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authn/casdoor"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/authz/spicedb"
	"github.com/aisphereio/kernel/cachex"
	_ "github.com/aisphereio/kernel/cachex/redis"
	"github.com/aisphereio/kernel/dbx"
	_ "github.com/aisphereio/kernel/dbx/postgres"
	"github.com/aisphereio/kernel/dtmx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/migrationx"
	"github.com/aisphereio/kernel/objectstorex"
	_ "github.com/aisphereio/kernel/objectstorex/minio"

	"github.com/aisphereio/aisphere-iam/internal/conf"
)

type ResourceOptions struct {
	Logger  logx.Logger
	Metrics metricsx.Manager
	DTM     dtmx.Manager
}

type Resources struct {
	DB           dbx.DB
	ControlPlane ControlPlaneRepository
	LocalUsers   LocalUserRepository
	Cache        cachex.Cache
	ObjectStore  objectstorex.Client
	Audit        auditx.Recorder
	Authn        authn.Authenticator
	Login        authn.LoginService
	Logout       authn.LogoutService
	Tokens       authn.TokenService
	Profile      authn.ProfileService
	Identity     authn.IdentityAdmin
	Authz        authz.Authorizer
	AuthzAdmin   authz.AdminProvider
	Access       accessx.Guard
	DTM          dtmx.Manager

	closers []func() error
}

type Data struct {
	Resources *Resources
}

func NewResources(ctx context.Context, cfg conf.Bootstrap, opts ResourceOptions) (*Resources, func(), error) {
	logger := opts.Logger
	if logger == nil {
		logger = logx.DefaultLogger()
	}
	metrics := metricsx.Ensure(opts.Metrics)

	r := &Resources{
		Audit: auditx.NewMemoryStore(),
		Authz: authz.DenyAll(),
		DTM:   dtmx.FromContextOr(ctx, opts.DTM),
	}
	if !cfg.Audit.Enabled {
		r.Audit = auditx.Noop()
	}
	if cfg.Security.Authz.DevAllowAll {
		r.Authz = authz.AllowAllForDevOnly()
	}

	if cfg.Data.Database.Enabled {
		dbCfg := cfg.Data.Database.Config
		dbCfg.Logger = logger.Named("data.dbx")
		dbCfg.Metrics = metrics
		dbCfg.MetricsEnabled = dbCfg.MetricsEnabled && cfg.Metrics.Enabled
		db, err := dbx.New(dbCfg)
		if err != nil {
			return nil, nil, err
		}
		r.DB = db
r.ControlPlane = NewControlPlaneRepository(db)
			r.LocalUsers = NewLocalUserRepository(db)
			r.closers = append(r.closers, db.Close)

		if cfg.Data.Migration.Enabled {
			migCfg := cfg.Data.Migration.Config
			if err := migrationx.Apply(ctx, db, migCfg); err != nil {
				r.Close()
				return nil, nil, err
			}
		}
	}
	if cfg.Data.Cache.Enabled {
		cacheCfg := cfg.Data.Cache.Config
		cacheCfg.Logger = logger.Named("data.cachex")
		cacheCfg.Metrics = metrics
		cacheCfg.MetricsEnabled = cacheCfg.MetricsEnabled && cfg.Metrics.Enabled
		cache, err := cachex.New(cacheCfg)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
		r.Cache = cache
		r.closers = append(r.closers, cache.Close)
	}
	if cfg.Data.ObjectStore.Enabled {
		storeCfg := cfg.Data.ObjectStore.Config
		storeCfg.Logger = logger.Named("data.objectstorex")
		storeCfg.Metrics = metrics
		storeCfg.MetricsEnabled = storeCfg.MetricsEnabled && cfg.Metrics.Enabled
		store, err := objectstorex.New(storeCfg)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
		r.ObjectStore = store
		r.closers = append(r.closers, store.Close)
	}
	if cfg.Security.Authn.Enabled {
		provider, err := newAuthenticator(cfg.Security.Authn, logger, metrics, cfg.Metrics.Enabled)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
r.Authn = provider
			r.Login = provider
			r.Logout = provider
			r.Tokens = provider
			r.Profile = provider
			r.Identity = provider
	}
	if cfg.Security.Authz.Enabled && !cfg.Security.Authz.DevAllowAll {
		provider, closeFn, err := newAuthorizer(cfg.Security.Authz, logger, metrics, cfg.Metrics.Enabled)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
		if cfg.Security.Authz.InstallDefaultSchema {
			if err := installSchema(ctx, provider, cfg.Security.Authz.SchemaPath); err != nil {
				r.Close()
				return nil, nil, err
			}
		}
		if err := bootstrapControlPlaneAdmins(ctx, cfg.ControlPlane.BootstrapAdmins, provider, logger); err != nil {
			r.Close()
			return nil, nil, err
		}
		r.Authz = provider
		r.AuthzAdmin = provider
		if closeFn != nil {
			r.closers = append(r.closers, closeFn)
		}
	}

	r.Access = accessx.New(r.Authn, r.Authz, r.Audit)
	return r, func() { _ = r.Close() }, pingEnabled(ctx, r)
}

func NewData(resources *Resources) *Data {
	return &Data{Resources: resources}
}

type identityProvider interface {
		authn.Authenticator
		authn.LoginService
		authn.LogoutService
		authn.TokenService
		authn.ProfileService
		authn.IdentityAdmin
	}

func newAuthenticator(cfg conf.AuthnConfig, logger logx.Logger, metrics metricsx.Manager, metricsEnabled bool) (identityProvider, error) {
	switch cfg.Provider {
	case "", "casdoor":
		casdoorCfg := cfg.Casdoor
		casdoorCfg.Logger = logger.Named("authn.casdoor")
		casdoorCfg.Metrics = metrics
		casdoorCfg.MetricsEnabled = casdoorCfg.MetricsEnabled && metricsEnabled
		client, err := casdoor.New(casdoorCfg)
		if err != nil {
			return nil, err
		}
		return newCasdoorClockSkewProvider(casdoorCfg, client), nil
	default:
		return nil, errors.New("unsupported authn provider: " + cfg.Provider)
	}
}

func newAuthorizer(cfg conf.AuthzConfig, logger logx.Logger, metrics metricsx.Manager, metricsEnabled bool) (*spicedb.Client, func() error, error) {
	switch cfg.Provider {
	case "", "spicedb":
		spiceCfg := cfg.SpiceDB
		spiceCfg.Logger = logger.Named("authz.spicedb")
		spiceCfg.Metrics = metrics
		spiceCfg.MetricsEnabled = spiceCfg.MetricsEnabled && metricsEnabled
		client, err := spicedb.New(spiceCfg)
		if err != nil {
			return nil, nil, err
		}
		return client, client.Close, nil
	default:
		return nil, nil, errors.New("unsupported authz provider: " + cfg.Provider)
	}
}

func installSchema(ctx context.Context, provider *spicedb.Client, schemaPath string) error {
	schemaPath = strings.TrimSpace(schemaPath)
	if schemaPath == "" {
		return provider.InstallDefaultSchema(ctx)
	}
	body, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	return provider.WriteSchema(ctx, authz.Schema{Text: string(body)})
}

func bootstrapControlPlaneAdmins(ctx context.Context, cfg conf.ControlPlaneBootstrapAdminsConfig, writer authz.RelationshipWriter, logger logx.Logger) error {
	if !cfg.Enabled || writer == nil || len(cfg.Subjects) == 0 {
		return nil
	}
	resources := cfg.Resources
	if len(resources) == 0 {
		resources = defaultControlPlaneAdminResources()
	}
	rels := make([]authz.Relationship, 0, len(resources)*len(cfg.Subjects))
	for _, resource := range resources {
		resource.Type = strings.TrimSpace(resource.Type)
		resource.ID = strings.TrimSpace(resource.ID)
		if resource.Type == "" || resource.ID == "" {
			continue
		}
		for _, subject := range cfg.Subjects {
			subject.Type = strings.TrimSpace(subject.Type)
			subject.ID = strings.TrimSpace(subject.ID)
			subject.Relation = strings.TrimSpace(subject.Relation)
			if subject.Type == "" || subject.ID == "" {
				continue
			}
			rels = append(rels, authz.Relationship{
				Resource: authz.ObjectRef{Type: resource.Type, ID: resource.ID},
				Relation: "admin",
				Subject:  authz.SubjectRef{Type: subject.Type, ID: subject.ID, Relation: subject.Relation},
			})
		}
	}
	if len(rels) == 0 {
		return nil
	}
	result, err := writer.WriteRelationships(ctx, rels...)
	if err != nil {
		return err
	}
	logger.Info("control plane admin relationships bootstrapped", logx.Int("written", result.Written))
	return nil
}

func defaultControlPlaneAdminResources() []conf.ControlPlaneAdminResource {
	return []conf.ControlPlaneAdminResource{
		{Type: "iam", ID: "organization"},
		{Type: "iam", ID: "capability"},
		{Type: "iam", ID: "resource_type"},
		{Type: "iam", ID: "resource"},
		{Type: "iam", ID: "resource_binding"},
		{Type: "iam", ID: "external_resource_binding"},
		{Type: "iam", ID: "role_template"},
		{Type: "iam", ID: "grant"},
	}
}

func pingEnabled(ctx context.Context, r *Resources) error {
	if r.DB != nil {
		if err := r.DB.PingContext(ctx); err != nil {
			return err
		}
	}
	if r.Cache != nil {
		if err := r.Cache.Ping(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resources) Close() error {
	var out error
	for i := len(r.closers) - 1; i >= 0; i-- {
		if err := r.closers[i](); err != nil && out == nil {
			out = err
		}
	}
	return out
}
