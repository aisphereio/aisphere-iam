package main

import (
	"context"
	"flag"
	"time"

	kernel "github.com/aisphereio/kernel"
	"github.com/aisphereio/kernel/configx"
	configenv "github.com/aisphereio/kernel/configx/env"
	"github.com/aisphereio/kernel/configx/file"
	"github.com/aisphereio/kernel/dtmx"
	_ "github.com/aisphereio/kernel/dtmx/dtm"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/taskx"
	taskxdapr "github.com/aisphereio/kernel/taskx/dapr"

	defaults "github.com/aisphereio/aisphere-iam/internal/biz/defaults"
	grantbiz "github.com/aisphereio/aisphere-iam/internal/biz/grant"
	projectbiz "github.com/aisphereio/aisphere-iam/internal/biz/project"
	"github.com/aisphereio/aisphere-iam/internal/biz/projection"
	resourcebiz "github.com/aisphereio/aisphere-iam/internal/biz/resource"
	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/server"
	"github.com/aisphereio/aisphere-iam/internal/service"
)

var (
	Name     = "app"
	Version  = "dev"
	flagconf string
)

func init() {
	flag.StringVar(&flagconf, "conf", "configs/config.yaml", "config path, eg: -conf configs/config.yaml")
}

func main() {
	flag.Parse()

	cfg := configx.New(configx.WithSource(file.NewSource(flagconf), configenv.NewSource()))
	defer cfg.Close()
	if err := cfg.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := cfg.Scan(&bc); err != nil {
		panic(err)
	}
	applyBuildInfo(&bc)

	logger, _, err := logx.New(bc.Log)
	if err != nil {
		panic(err)
	}
	defer func() { _ = logger.Sync() }()

	metrics := metricsx.Noop()
	if bc.Metrics.Enabled {
		metrics = metricsx.NewPrometheusManager(bc.Service.Name, bc.Service.Version, logger)
	}

	dtmManager, err := newDTMManager(bc, logger, metrics)
	if err != nil {
		panic(err)
	}
	defer func() { _ = dtmManager.Close() }()

	resources, cleanup, err := data.NewResources(context.Background(), bc, data.ResourceOptions{Logger: logger, Metrics: metrics, DTM: dtmManager})
	if err != nil {
		panic(err)
	}
	defer cleanup()

	projectionManager := projection.NewManager(resources.ControlPlane, resources.AuthzAdmin, resources.DTM)
	projectUsecase := projectbiz.NewService(resources.ControlPlane, resources.AuthzAdmin, projectionManager)
	resourceUsecase := resourcebiz.NewService(resources.ControlPlane, resources.AuthzAdmin, projectionManager)
	grantUsecase := grantbiz.NewService(resources.ControlPlane, resources.Authz, resources.AuthzAdmin, projectionManager)
	if bc.ControlPlane.Defaults.Enabled {
		if _, err := defaults.ReconcileFile(context.Background(), bc.ControlPlane.Defaults.Path, defaults.Services{Projects: projectUsecase, Resources: resourceUsecase, Grants: grantUsecase}); err != nil {
			panic(err)
		}
	}

	service.ConfigureExternalAuthInternalCall(bc.Security.InternalCall)
	deps := service.IAMDeps{Tokens: resources.Tokens, Identity: resources.Identity, Authz: resources.AuthzAdmin}
authService := service.NewIAMAuthService(deps)
		directoryService := service.NewIAMDirectoryService(deps)
		groupService := service.NewIAMGroupAdminService(deps)
		permissionService := service.NewIAMPermissionService(deps)
		authzAdminService := service.NewIAMAuthorizationAdminService(deps)
		projectService := service.NewProjectService(projectUsecase, resources.ControlPlane)
		resourceService := service.NewResourceService(resourceUsecase, resources.ControlPlane)
		grantService := service.NewGrantService(grantUsecase, resources.ControlPlane)

httpServer := server.NewHTTPServer(bc.Server, bc.Log, bc.Metrics, logger, metrics, resources, projectionManager, authService, directoryService, groupService, permissionService, authzAdminService, projectService, resourceService, grantService, bc.Security)
			grpcServer := server.NewGRPCServer(bc.Server, bc.Log, bc.Metrics, logger, metrics, resources, authService, directoryService, groupService, permissionService, authzAdminService, projectService, resourceService, grantService, bc.Security)

		// ── taskx / Dapr Jobs ──────────────────────────────────────────────
		// Dapr callback server on a dedicated port so sidecar callbacks bypass
		// the IAM gRPC authn/authz middleware.
		taskRuntime, taskCallback, err := taskxdapr.NewStandalone(":19081")
		if err != nil {
			panic(err)
		}
		defer taskRuntime.Close()

		// Register the grant expiration reconciler handler.
		if err := taskRuntime.RegisterHandler("grant-expiration-reconciler",
			func(ctx context.Context, event taskx.TriggerEvent) error {
				return grantUsecase.ExpireDueGrants(ctx)
			},
		); err != nil {
			panic(err)
		}

		// Schedule the job: run every 5 minutes, overwrite on each boot so all
		// replicas share the same definition.
		maxRetries := uint32(5)
		if err := taskRuntime.Schedule(context.Background(), taskx.ManagedJob{
			Name:      "grant-expiration-reconciler",
			Schedule:  "@every 5m",
			Overwrite: true,
			Data:      []byte(`{"batch_size":100}`),
			DataTypeURL: "application/json",
			FailurePolicy: &taskx.DeliveryFailurePolicy{
				Mode:       taskx.DeliveryFailureConstant,
				MaxRetries: &maxRetries,
				Interval:   5 * time.Second,
			},
		}); err != nil {
			panic(err)
		}

		options := []kernel.Option{
			kernel.Name(bc.Service.Name),
			kernel.Version(bc.Service.Version),
			kernel.LogxLogger(logger),
			kernel.Metrics(metrics),
			kernel.DTM(dtmManager),
			kernel.Server(httpServer, grpcServer, taskCallback),
			kernel.StopTimeout(10 * time.Second),
		}
	if bc.Metrics.Enabled && bc.Metrics.Addr != "" {
		options = append(options, kernel.PrometheusMetrics(bc.Metrics.Addr), kernel.MetricsPath(bc.Metrics.Path), kernel.MetricsPprof(bc.Metrics.Pprof))
	}
	options = append(options, kernel.MetricsSystem(bc.Metrics.Enabled && bc.Metrics.Runtime))

	app := kernel.New(options...)
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func applyBuildInfo(bc *conf.Bootstrap) {
	if bc.Service.Name == "" {
		bc.Service.Name = Name
	}
	if bc.Service.Version == "" {
		bc.Service.Version = Version
	}
	if bc.Service.Env == "" {
		bc.Service.Env = "local"
	}
	if bc.Log.ServiceName == "" {
		bc.Log.ServiceName = bc.Service.Name
	}
	if bc.Log.Env == "" {
		bc.Log.Env = bc.Service.Env
	}
	if bc.Log.Version == "" {
		bc.Log.Version = bc.Service.Version
	}
	if bc.Metrics.Path == "" {
		bc.Metrics.Path = "/metrics"
	}
	if bc.DTM.ServiceBaseURL == "" && bc.Server.HTTP.Addr != "" {
		bc.DTM.ServiceBaseURL = "http://127.0.0.1" + normalizeAddrPort(bc.Server.HTTP.Addr)
	}
	if bc.DTM.BranchPrefix == "" {
		bc.DTM.BranchPrefix = "/internal/dtm"
	}
}

func newDTMManager(bc conf.Bootstrap, logger logx.Logger, metrics metricsx.Manager) (dtmx.Manager, error) {
	cfg := bc.DTM
	cfg.Logger = logger.Named("dtmx")
	cfg.Metrics = metrics
	cfg.MetricsEnabled = cfg.MetricsEnabled && bc.Metrics.Enabled
	return dtmx.New(cfg)
}

func normalizeAddrPort(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i:]
		}
	}
	return ":8000"
}
