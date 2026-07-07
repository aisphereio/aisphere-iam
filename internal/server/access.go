package server

import (
	"context"
	"strings"

	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/middleware"
	accessmw "github.com/aisphereio/kernel/middleware/access"
	"github.com/aisphereio/kernel/securityx"
	"github.com/aisphereio/kernel/serverx"
)

func iamServerMiddlewares(resources *data.Resources, cfg conf.SecurityConfig) []middleware.Middleware {
	if resources == nil {
		return nil
	}
	catalog := IAMCatalog()
	securityRuntime := mustSecurityRuntime(cfg)
	providers := catalog.RuntimeProviders(serverx.RuntimeProviders{
		Security:    securityRuntime,
		AccessGuard: &resources.Access,
	})
	providers.SkipPolicyResolver = iamSkipPolicyResolver(catalog)
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
		Access:       accessx.AccessConfig{},
	}, nil)
	if err != nil {
		panic(err)
	}
	return runtime
}

func iamSkipPolicyResolver(catalog serverx.ServiceCatalog) accessmw.SkipPolicyResolver {
	return func(operation string) accessx.SkipPolicy {
		op := strings.TrimSpace(operation)
		switch strings.TrimPrefix(op, "/") {
		case "healthz", "readyz", "metrics":
			return accessx.SkipAll
		}
		info, ok, err := catalog.RequestInfoResolver(context.Background(), op, nil)
		if err == nil && ok {
			if info.Exposure == accessv1.Exposure_PUBLIC {
				return accessx.SkipAll
			}
			if strings.EqualFold(info.Labels["authz_mode"], "SELF_CHECK") {
				return accessx.SkipAuthz
			}
		}
		switch op {
		case "CreateOrganization", "iam.project.v1.ProjectService/CreateOrganization", "/iam.project.v1.ProjectService/CreateOrganization":
			return accessx.SkipAuthz
		}
		return accessx.SkipDefault
	}
}
