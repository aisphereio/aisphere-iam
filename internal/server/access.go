package server

import (
	"context"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/middleware"
	mwaccess "github.com/aisphereio/kernel/middleware/access"
	mwauthn "github.com/aisphereio/kernel/middleware/authn"
	"github.com/aisphereio/kernel/middleware/requestinfo"
	"github.com/aisphereio/kernel/requestx"
)

func iamServerMiddlewares(resources *data.Resources) []middleware.Middleware {
	if resources == nil {
		return nil
	}
	return []middleware.Middleware{
		requestinfo.Server(requestinfo.WithResolver(iamRequestInfoResolver)),
		mwauthn.Server(
			mwauthn.WithAuthenticator(resources.Authn),
			mwauthn.WithAllowAnonymous(true),
		),
		mwaccess.Server(resources.Access, mwaccess.WithResolver(iamAccessResolver)),
	}
}

func iamRequestInfoResolver(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
	resolvers := []requestx.Resolver{
		v1.IAMAuthServiceKernelRequestInfoResolver,
		v1.IAMDirectoryServiceKernelRequestInfoResolver,
		v1.IAMPermissionServiceKernelRequestInfoResolver,
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
		v1.IAMPermissionServiceKernelAccessResolver,
	}
	for _, resolver := range resolvers {
		check, ok, err := resolver(ctx, operation, req)
		if err != nil || ok {
			return check, ok, err
		}
	}
	return accessx.Check{}, false, nil
}
