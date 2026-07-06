package server

import (
	"context"

	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/middleware"
	"github.com/aisphereio/kernel/securityx"
	"github.com/aisphereio/kernel/serverx"
)

func iamServerMiddlewares(resources *data.Resources, cfg conf.SecurityConfig) []middleware.Middleware {
	if resources == nil {
		return nil
	}
	securityRuntime := mustSecurityRuntime(cfg)
	providers := IAMCatalog().RuntimeProviders(serverx.RuntimeProviders{
		Security:    securityRuntime,
		AccessGuard: &resources.Access,
	})
	return serverx.ServerMiddlewareFromProviders(context.Background(), providers)
}

func mustSecurityRuntime(cfg conf.SecurityConfig) *securityx.Runtime {
	runtime, err := securityx.NewRuntime(context.Background(), securityx.Config{
		Authn: securityx.AuthnBoundaryConfig{
			Enabled:        cfg.Authn.Enabled,
			Mode:           cfg.Authn.Mode,
			Provider:       cfg.Authn.Provider,
			OIDC:           cfg.Authn.OIDC,
			CacheTTL:       cfg.Authn.CacheTTL,
			InternalCall:   cfg.InternalCall,
			AllowAnonymous: true,
		},
		InternalCall: cfg.InternalCall,
		Access: accessx.AccessConfig{
			SkipOperations:     cfg.Access.SkipOperations,
			PublicOperations:   cfg.Access.PublicOperations,
			AllowAllOperations: cfg.Access.AllowAllOperations,
		},
	}, nil)
	if err != nil {
		panic(err)
	}
	return runtime
}
