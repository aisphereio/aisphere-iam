package server

import (
	grantv1 "github.com/aisphereio/aisphere-iam/api/iam/grant/v1"
	projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/service"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	kgrpc "github.com/aisphereio/kernel/transportx/grpc"
)

func NewGRPCServer(c conf.ServerConfig, logCfg logx.Config, metricsCfg conf.MetricsConfig, logger logx.Logger, metrics metricsx.Manager, resources *data.Resources, authSvc *service.IAMAuthService, dirSvc *service.IAMDirectoryService, permSvc *service.IAMPermissionService, projectSvc *service.ProjectService, resourceSvc *service.ResourceService, grantSvc *service.GrantService, securityCfg conf.SecurityConfig) *kgrpc.Server {
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
	v1.RegisterIAMAuthServiceServer(srv, authSvc)
	v1.RegisterIAMDirectoryServiceServer(srv, dirSvc)
	v1.RegisterIAMPermissionServiceServer(srv, permSvc)
	projectv1.RegisterProjectServiceServer(srv, projectSvc)
	resourcev1.RegisterResourceServiceServer(srv, resourceSvc)
	grantv1.RegisterGrantServiceServer(srv, grantSvc)
	return srv
}
