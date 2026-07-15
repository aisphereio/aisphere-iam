package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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
	"github.com/aisphereio/aisphere-iam/internal/permissionmanifest"
)

type ResourceOptions struct {
	Logger  logx.Logger
	Metrics metricsx.Manager
	DTM     dtmx.Manager
}

type Resources struct {
	DB                 dbx.DB
	ControlPlane       ControlPlaneRepository
	Cache              cachex.Cache
	ObjectStore        objectstorex.Client
	Audit              auditx.Recorder
	Authn              authn.Authenticator
	Tokens             authn.TokenService
	Identity           authn.IdentityAdmin
	Authz              authz.Authorizer
	AuthzAdmin         authz.AdminProvider
	Access             accessx.Guard
	DTM                dtmx.Manager
	IdentityProjection *IdentityProjectionDispatcher
	PermissionManifest *permissionmanifest.Manifest

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
		Authz: authz.DenyAll(),
		DTM:   dtmx.FromContextOr(ctx, opts.DTM),
	}
	manifest, err := loadPermissionManifest(cfg)
	if err != nil {
		return nil, nil, err
	}
	r.PermissionManifest = manifest
	if !cfg.Audit.Enabled {
		r.Audit = auditx.Noop()
	} else {
		r.Audit = auditx.NewMemoryStore()
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
		r.closers = append(r.closers, db.Close)

		// Use PostgreSQL audit store when database is available
		if cfg.Audit.Enabled && cfg.Audit.Store == "postgres" {
			auditStore := auditx.NewPostgresStore(db)
			if err := auditStore.EnsureTable(ctx); err != nil {
				logger.Warn("failed to ensure audit table, falling back to memory", logx.Any("error", err))
			} else {
				r.Audit = auditStore
				logger.Info("audit store set to postgres")
			}
		}

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
		identity, err := identityForMode(cfg.Security.Authn.IdentityMode, provider)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
		r.Authn = provider
		r.Tokens = provider
		r.Identity = identity
	}
	if cfg.Security.Authz.Enabled && !cfg.Security.Authz.DevAllowAll {
		provider, closeFn, err := newAuthorizer(cfg.Security.Authz, logger, metrics, cfg.Metrics.Enabled)
		if err != nil {
			r.Close()
			return nil, nil, err
		}
		r.Authz = provider
		r.AuthzAdmin = provider
		if cfg.Security.Authz.InstallDefaultSchema {
			if err := BootstrapAuthzSchema(ctx, cfg.Security.Authz, provider, logger); err != nil {
				r.Close()
				return nil, nil, err
			}
		}
		bootstrapPolicy := permissionmanifest.BootstrapPolicy{}
		if r.PermissionManifest != nil {
			bootstrapPolicy = r.PermissionManifest.Bootstrap
		}
		if err := bootstrapControlPlaneAdmins(ctx, cfg.ControlPlane.BootstrapAdmins, bootstrapPolicy, provider, r.Identity, logger); err != nil {
			r.Close()
			return nil, nil, err
		}
		if r.Identity != nil {
			r.IdentityProjection = NewIdentityProjectionDispatcher(provider, provider, r.DTM, r.DB)
			if err := r.IdentityProjection.EnsureStore(ctx); err != nil {
				r.Close()
				return nil, nil, err
			}
			if r.DB != nil {
				retryCtx, cancel := context.WithCancel(context.Background())
				r.closers = append(r.closers, func() error { cancel(); return nil })
				go r.IdentityProjection.StartRetryWorker(retryCtx, time.Minute)
			}
			r.Identity = BindIdentityAuthZ(r.Identity, provider, WithIdentityProjectionDispatcher(r.IdentityProjection), WithIdentityProjectionReader(provider))
		}
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

func loadPermissionManifest(cfg conf.Bootstrap) (*permissionmanifest.Manifest, error) {
	if !cfg.ControlPlane.Defaults.Enabled && !cfg.ControlPlane.BootstrapAdmins.Enabled {
		return nil, nil
	}
	path := strings.TrimSpace(cfg.ControlPlane.Defaults.Path)
	if path == "" {
		return nil, fmt.Errorf("control_plane.defaults.path is required for permission bootstrap")
	}
	manifest, err := permissionmanifest.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load permission manifest %s: %w", path, err)
	}
	if !cfg.Security.Authz.InstallDefaultSchema {
		return manifest, nil
	}
	schemaPath := strings.TrimSpace(cfg.Security.Authz.SchemaPath)
	if schemaPath == "" {
		return nil, fmt.Errorf("security.authz.schema_path is required for permission manifest validation")
	}
	body, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read authz schema %s: %w", schemaPath, err)
	}
	schema, err := permissionmanifest.ParseSchema(string(body))
	if err != nil {
		return nil, fmt.Errorf("parse authz schema %s: %w", schemaPath, err)
	}
	if err := permissionmanifest.Validate(manifest, schema); err != nil {
		return nil, fmt.Errorf("validate permission manifest: %w", err)
	}
	return manifest, nil
}

func bootstrapControlPlaneAdmins(ctx context.Context, cfg conf.ControlPlaneBootstrapAdminsConfig, policy permissionmanifest.BootstrapPolicy, writer authz.RelationshipWriter, directory authn.UserDirectory, logger logx.Logger) error {
	if !cfg.Enabled || writer == nil || len(cfg.Subjects) == 0 {
		return nil
	}
	platformID := strings.TrimSpace(policy.PlatformID)
	if platformID == "" {
		return fmt.Errorf("bootstrap platform_id is required")
	}
	rels := make([]authz.Relationship, 0, len(policy.PlatformResources)+len(cfg.Subjects)*2)
	for _, resource := range policy.PlatformResources {
		resource.Type = strings.TrimSpace(resource.Type)
		resource.ID = strings.TrimSpace(resource.ID)
		if resource.Type == "" || resource.ID == "" {
			continue
		}
		rels = append(rels, authz.Relationship{
			Resource: authz.ObjectRef{Type: resource.Type, ID: resource.ID},
			Relation: "platform",
			Subject:  authz.SubjectRef{Type: "platform", ID: platformID},
		})
	}
	for _, subject := range cfg.Subjects {
		rolePolicy, canonicalRole, ok := policy.ResolveRole(subject.Role)
		if !ok {
			return fmt.Errorf("unknown bootstrap role %s", strings.TrimSpace(subject.Role))
		}
		resolvedSubject, hasSubject, err := resolveBootstrapZoneSubject(ctx, subject, directory)
		if err != nil {
			return err
		}
		if !hasSubject {
			continue
		}
		zoneID := dataFirstNonEmpty(subject.ZoneID, subject.CasdoorOrg)
		var resource authz.ObjectRef
		switch strings.TrimSpace(rolePolicy.Scope) {
		case "platform":
			resource = authz.ObjectRef{Type: "platform", ID: platformID}
		case "zone":
			if zoneID == "" {
				return fmt.Errorf("zone_id is required for bootstrap role %s", canonicalRole)
			}
			resource = authz.ObjectRef{Type: "zone", ID: zoneID}
		case "":
			return fmt.Errorf("bootstrap role %s scope is required", canonicalRole)
		default:
			return fmt.Errorf("unsupported bootstrap role scope %s", rolePolicy.Scope)
		}
		if zoneID != "" {
			rels = append(rels, authz.Relationship{
				Resource: authz.ObjectRef{Type: "zone", ID: zoneID},
				Relation: "platform",
				Subject:  authz.SubjectRef{Type: "platform", ID: platformID},
			})
		}
		rels = append(rels, authz.Relationship{Resource: resource, Relation: strings.TrimSpace(rolePolicy.Relation), Subject: resolvedSubject})
	}
	rels = dedupeRelationships(rels)
	if len(rels) == 0 {
		return nil
	}
	result, err := writer.WriteRelationships(ctx, rels...)
	if err != nil {
		return err
	}
	logger.Info("control plane admin relationships bootstrapped", logx.Int("written", result.Written))
	if cfg.CleanupLegacyExpansions {
		deleted, err := cleanupLegacyBootstrapExpansions(ctx, cfg.Subjects, policy, writer, directory)
		if err != nil {
			return err
		}
		logger.Info("legacy control plane admin relationships cleaned", logx.Int("deleted", deleted))
	}
	return nil
}

// legacyBootstrapZoneRelations maps each canonical bootstrap role to the zone
// relations that the pre-convergence bootstrap code used to expand it into.
// platform_owner / platform_admin are included because those subjects were
// previously bootstrapped as zone_owner / zone_admin and carry the same legacy
// zone-level expansions that must be cleaned up now that a single platform
// relationship supersedes them.  platform_admin maps to the full zone_owner
// expansion (including owner) because the typical migration path was
// zone_owner → platform_admin.
var legacyBootstrapZoneRelations = map[string][]string{
	"zone_owner":     {"owner", "admin", "user_manager", "group_manager", "permission_admin"},
	"zone_admin":     {"admin", "user_manager", "group_manager", "permission_admin"},
	"platform_owner": {"owner", "admin", "user_manager", "group_manager", "permission_admin"},
	"platform_admin": {"owner", "admin", "user_manager", "group_manager", "permission_admin"},
}

func cleanupLegacyBootstrapExpansions(ctx context.Context, subjects []conf.ControlPlaneAdminSubject, policy permissionmanifest.BootstrapPolicy, writer authz.RelationshipWriter, directory authn.UserDirectory) (int, error) {
	deleted := 0
	for _, subject := range subjects {
		role, canonicalRole, ok := policy.ResolveRole(subject.Role)
		if !ok {
			return deleted, fmt.Errorf("unknown bootstrap role %s", strings.TrimSpace(subject.Role))
		}
		resolvedSubject, hasSubject, err := resolveBootstrapZoneSubject(ctx, subject, directory)
		if err != nil {
			return deleted, err
		}
		if !hasSubject {
			continue
		}
		zoneID := dataFirstNonEmpty(subject.ZoneID, subject.CasdoorOrg)
		for _, relation := range legacyBootstrapZoneRelations[canonicalRole] {
			if role.Scope == "zone" && relation == strings.TrimSpace(role.Relation) {
				continue
			}
			part, err := writer.DeleteRelationships(ctx, authz.RelationshipFilter{
				ResourceType: "zone", ResourceID: zoneID, Relation: relation,
				SubjectType: resolvedSubject.Type, SubjectID: resolvedSubject.ID, SubjectRel: resolvedSubject.Relation,
			})
			deleted += part.Deleted
			if err != nil {
				return deleted, err
			}
		}
		// Clean up legacy iam#admin / iam_authz#admin relationships that the
		// old bootstrap wrote for every platform resource.  This now runs for
		// all roles that have legacy zone expansions (including platform_owner
		// and platform_admin), not just zone_owner / zone_admin.
		for _, resource := range policy.PlatformResources {
			part, err := writer.DeleteRelationships(ctx, authz.RelationshipFilter{
				ResourceType: strings.TrimSpace(resource.Type), ResourceID: strings.TrimSpace(resource.ID), Relation: "admin",
				SubjectType: resolvedSubject.Type, SubjectID: resolvedSubject.ID, SubjectRel: resolvedSubject.Relation,
			})
			deleted += part.Deleted
			if err != nil {
				return deleted, err
			}
		}
	}
	return deleted, nil
}

func resolveBootstrapZoneSubject(ctx context.Context, subject conf.ControlPlaneAdminSubject, directory authn.UserDirectory) (authz.SubjectRef, bool, error) {
	subjectType := strings.TrimSpace(subject.Type)
	if subjectType == "" {
		subjectType = authz.SubjectTypeUser
	}
	relation := strings.TrimSpace(subject.Relation)
	if subjectID := dataFirstNonEmpty(subject.ID, subject.ExternalSubject); subjectID != "" {
		return authz.SubjectRef{Type: subjectType, ID: subjectID, Relation: relation}, true, nil
	}

	username := strings.TrimSpace(subject.Username)
	if username == "" {
		return authz.SubjectRef{}, false, nil
	}
	if directory == nil {
		return authz.SubjectRef{}, false, authn.ErrIdentityBackendFailed("bootstrap admin username requires identity provider", nil)
	}
	orgID := dataFirstNonEmpty(subject.CasdoorOrg, subject.ZoneID)
	users, err := directory.FindUsers(ctx, authn.UserFilter{OrgID: orgID, Username: username, Limit: 20})
	if err != nil {
		return authz.SubjectRef{}, false, authn.ErrIdentityBackendFailed("resolve bootstrap admin user failed", err)
	}
	for _, user := range users {
		if !strings.EqualFold(user.Username, username) {
			continue
		}
		if user.ID == "" {
			return authz.SubjectRef{}, false, authn.ErrIdentityBackendFailed("bootstrap admin user has empty stable id", nil)
		}
		return authz.SubjectRef{Type: subjectType, ID: user.ID, Relation: relation}, true, nil
	}
	return authz.SubjectRef{}, false, authn.ErrIdentityBackendFailed("bootstrap admin user not found: "+orgID+"/"+username, nil)
}

func dataFirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
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
