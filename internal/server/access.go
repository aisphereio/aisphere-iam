package server

import (
	"context"

	grantv1 "github.com/aisphereio/aisphere-iam/api/iam/grant/v1"
	projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/middleware"
	mwaccess "github.com/aisphereio/kernel/middleware/access"
	"github.com/aisphereio/kernel/requestx"
	"github.com/aisphereio/kernel/securityx"
	"github.com/aisphereio/kernel/serverx"
)

func iamServerMiddlewares(resources *data.Resources, cfg conf.SecurityConfig) []middleware.Middleware {
	if resources == nil {
		return nil
	}
	securityRuntime := mustSecurityRuntime(cfg)
	return serverx.ServerMiddlewareFromProviders(context.Background(), serverx.RuntimeProviders{
		Security:            securityRuntime,
		AccessGuard:         &resources.Access,
		RequestInfoResolver: iamRequestInfoResolver,
		AccessResolver:      iamAccessResolver,
	})
}

func mustSecurityRuntime(cfg conf.SecurityConfig) *securityx.Runtime {
	runtime, err := securityx.NewRuntime(context.Background(), securityx.Config{
		Authn: securityx.AuthnBoundaryConfig{
			Enabled:      cfg.Authn.Enabled,
			Mode:         cfg.Authn.Mode,
			Provider:     cfg.Authn.Provider,
			OIDC:         cfg.Authn.OIDC,
			CacheTTL:     cfg.Authn.CacheTTL,
			InternalCall: cfg.InternalCall,
			// Public endpoints are selected by security.access.public_operations.
			// Authn therefore allows anonymous requests and lets accessx decide
			// whether anonymous is acceptable for the current operation.
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

func iamRequestInfoResolver(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
	resolvers := []requestx.Resolver{
		v1.IAMAuthServiceKernelRequestInfoResolver,
		v1.IAMDirectoryServiceKernelRequestInfoResolver,
		v1.IAMIdentityAdminServiceKernelRequestInfoResolver,
		v1.IAMPermissionServiceKernelRequestInfoResolver,
		projectv1.ProjectServiceKernelRequestInfoResolver,
		resourcev1.ResourceServiceKernelRequestInfoResolver,
		grantv1.GrantServiceKernelRequestInfoResolver,
	}
	for _, resolver := range resolvers {
		info, ok, err := resolver(ctx, operation, req)
		if err != nil || ok {
			return info, ok, err
		}
	}
	return requestx.Info{}, false, nil
}

func iamAccessResolver(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
	resolvers := []mwaccess.Resolver{
		v1.IAMAuthServiceKernelAccessResolver,
		v1.IAMDirectoryServiceKernelAccessResolver,
		v1.IAMIdentityAdminServiceKernelAccessResolver,
		v1.IAMPermissionServiceKernelAccessResolver,
		projectv1.ProjectServiceKernelAccessResolver,
		resourcev1.ResourceServiceKernelAccessResolver,
		grantv1.GrantServiceKernelAccessResolver,
	}
	for _, resolver := range resolvers {
		check, ok, err := resolver(ctx, operation, req)
		if err != nil || ok {
			return check, ok, err
		}
	}
	return accessx.Check{}, false, nil
}

var _ requestx.Resolver = iamRequestInfoResolver
var _ mwaccess.Resolver = iamAccessResolver
