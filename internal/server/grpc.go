package server

import (
	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/service"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/serverx"
	kgrpc "github.com/aisphereio/kernel/transportx/grpc"
)

func NewGRPCServer(c conf.ServerConfig, logCfg logx.Config, metricsCfg conf.MetricsConfig, logger logx.Logger, metrics metricsx.Manager, resources *data.Resources, authSvc *service.IAMAuthService, dirSvc *service.IAMDirectoryService, groupSvc *service.IAMGroupAdminService, permSvc *service.IAMPermissionService, authzAdminSvc *service.IAMAuthorizationAdminService, projectSvc *service.ProjectService, resourceSvc *service.ResourceService, grantSvc *service.GrantService, accessQuerySvc *service.AccessQueryService, securityCfg conf.SecurityConfig) *kgrpc.Server {
	var opts []kgrpc.ServerOption
	if c.GRPC.Addr != "" {
		opts = append(opts, kgrpc.Address(c.GRPC.Addr))
	}
	if c.GRPC.Timeout > 0 {
		opts = append(opts, kgrpc.Timeout(c.GRPC.Timeout))
	}
	opts = append(opts,
		kgrpc.Logger(logger.Named("transport.grpc")),
		kgrpc.AccessLog(logCfg.AccessLog),
	)
	if metricsCfg.Enabled {
		opts = append(opts, kgrpc.Metrics(metrics))
	}
	if m := iamServerMiddlewares(resources, securityCfg); len(m) > 0 {
		opts = append(opts, kgrpc.Middleware(m...))
	}
	srv := kgrpc.NewServer(opts...)
	if err := serverx.RegisterGRPCServices(srv, IAMBindings(resources, authSvc, dirSvc, groupSvc, permSvc, projectSvc, resourceSvc, grantSvc, accessQuerySvc)...); err != nil {
		panic(err)
	}
	v1.RegisterIAMAuthorizationAdminServiceServer(srv, authzAdminSvc)
	return srv
}
