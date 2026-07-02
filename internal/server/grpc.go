package server

import (
	v1 "aisphere-iam/api/iam/v1"
	"aisphere-iam/internal/conf"
	"aisphere-iam/internal/service"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	kgrpc "github.com/aisphereio/kernel/transportx/grpc"
)

func NewGRPCServer(c conf.ServerConfig, logCfg logx.Config, metricsCfg conf.MetricsConfig, logger logx.Logger, metrics metricsx.Manager, authSvc *service.IAMAuthService, dirSvc *service.IAMDirectoryService, permSvc *service.IAMPermissionService) *kgrpc.Server {
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
	srv := kgrpc.NewServer(opts...)
	v1.RegisterIAMAuthServiceServer(srv, authSvc)
	v1.RegisterIAMDirectoryServiceServer(srv, dirSvc)
	v1.RegisterIAMPermissionServiceServer(srv, permSvc)
	return srv
}
