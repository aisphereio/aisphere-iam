package main

import (
	"context"
	"flag"
	"time"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	kernel "github.com/aisphereio/kernel"
	"github.com/aisphereio/kernel/configx"
	configenv "github.com/aisphereio/kernel/configx/env"
	"github.com/aisphereio/kernel/configx/file"
	"github.com/aisphereio/kernel/dtmx"
	_ "github.com/aisphereio/kernel/dtmx/dtm"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/serverx"

	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/registry"
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

	resources, cleanup, err := data.NewResources(context.Background(), bc, data.ResourceOptions{
		Logger:  logger,
		Metrics: metrics,
		DTM:     dtmManager,
	})
	if err != nil {
		panic(err)
	}
	defer cleanup()

	deps := service.IAMDeps{
		Login:    resources.Login,
		Tokens:   resources.Tokens,
		Profile:  resources.Profile,
		Identity: resources.Identity,
		Authz:    resources.AuthzAdmin,
	}
	authService := service.NewIAMAuthService(deps)
	directoryService := service.NewIAMDirectoryService(deps)
	permissionService := service.NewIAMPermissionService(deps)
	if bc.Gateway.RouteRegistry.Provider != "" || len(bc.Gateway.RouteRegistry.Endpoints) > 0 {
		routeRegistry, registryCleanup, err := registry.NewRouteRegistry(context.Background(), registry.Config{
			Provider:       bc.Gateway.RouteRegistry.Provider,
			Prefix:         bc.Gateway.RouteRegistry.Prefix,
			Endpoints:      bc.Gateway.RouteRegistry.Endpoints,
			DialTimeout:    bc.Gateway.RouteRegistry.DialTimeout,
			RequestTimeout: bc.Gateway.RouteRegistry.RequestTimeout,
		})
		if err != nil {
			panic(err)
		}
		defer registryCleanup()
		if err := serverx.RegisterServiceGatewayRoutes(context.Background(), routeRegistry,
			v1.IAMAuthServiceKernelModule(),
			v1.IAMDirectoryServiceKernelModule(),
			v1.IAMPermissionServiceKernelModule(),
		); err != nil {
			panic(err)
		}
	}
	httpServer := server.NewHTTPServer(bc.Server, bc.Log, bc.Metrics, logger, metrics, resources, authService, directoryService, permissionService)
	grpcServer := server.NewGRPCServer(bc.Server, bc.Log, bc.Metrics, logger, metrics, resources, authService, directoryService, permissionService)

	options := []kernel.Option{
		kernel.Name(bc.Service.Name),
		kernel.Version(bc.Service.Version),
		kernel.LogxLogger(logger),
		kernel.Metrics(metrics),
		kernel.DTM(dtmManager),
		kernel.Server(httpServer, grpcServer),
		kernel.StopTimeout(10 * time.Second),
	}
	if bc.Metrics.Enabled && bc.Metrics.Addr != "" {
		options = append(options,
			kernel.PrometheusMetrics(bc.Metrics.Addr),
			kernel.MetricsPath(bc.Metrics.Path),
			kernel.MetricsPprof(bc.Metrics.Pprof),
		)
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
