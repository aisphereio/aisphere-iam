package server

import (
	"context"
	"strings"

	grantv1 "github.com/aisphereio/aisphere-iam/api/iam/grant/v1"
	projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/accessx"
	"github.com/aisphereio/kernel/middleware"
	mwaccess "github.com/aisphereio/kernel/middleware/access"
	mwauthn "github.com/aisphereio/kernel/middleware/authn"
	"github.com/aisphereio/kernel/middleware/requestinfo"
	"github.com/aisphereio/kernel/requestx"
)

func iamServerMiddlewares(resources *data.Resources, cfg conf.SecurityConfig) []middleware.Middleware {
	if resources == nil {
		return nil
	}
	return []middleware.Middleware{
		requestinfo.Server(requestinfo.WithResolver(iamRequestInfoResolver)),
		mwauthn.Server(
			mwauthn.WithAuthenticator(resources.Authn),
			mwauthn.WithAllowAnonymous(true),
		),
		mwaccess.Server(resources.Access, mwaccess.WithResolver(newIAMAccessResolver(cfg))),
	}
}

func iamRequestInfoResolver(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
	resolvers := []requestx.Resolver{
		v1.IAMAuthServiceKernelRequestInfoResolver,
		v1.IAMDirectoryServiceKernelRequestInfoResolver,
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

// newIAMAccessResolver returns a resolver that maps operations to accessx.Check.
// It reads the security config to decide which operations are allow-all
// (any authenticated user can perform them) vs. requiring SpiceDB authorization.
//
// The skip/allow-all set is configurable per deployment via the access config:
//
//	security:
//	  access:
//	    skip_operations:
//	      - GetMe
//	      - CreateOrganization
//	    allow_all_operations:   # deprecated, use skip_operations instead
//	      - CreateOrganization
//	      - GetMe
//
// Operators can remove an operation from this list to require platform-level
// authorization without code changes.
func newIAMAccessResolver(security conf.SecurityConfig) mwaccess.Resolver {
	// Convert the service-level AccessConfig to the kernel-level accessx.AccessConfig
	// and create a SkipPolicyResolver for config-driven skip policies.
	skipResolver := accessx.NewSkipPolicyResolver(accessx.AccessConfig{
		SkipOperations:     security.Access.SkipOperations,
		PublicOperations:   security.Access.PublicOperations,
		AllowAllOperations: security.Access.AllowAllOperations,
	})

	return func(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
		// 1. Check skip policy first (handles both SkipAuthz and SkipAll).
		if policy := skipResolver(operation); policy != accessx.SkipDefault {
			check := accessx.Check{
				SkipPolicy:  policy,
				AuditAction: resolveAuditAction(operation),
			}
			return check, true, nil
		}

		// 2. Fallback: check hardcoded operations for backward compatibility
		// with the old AuthzModeNone + AllowAll pattern.
		if isGetMeOperation(operation) {
			check := accessx.Check{
				SkipPolicy:  accessx.SkipAuthz,
				AuditAction: "iam.get_me",
			}
			return check, true, nil
		}
		if isCreateOrganizationOperation(operation) {
			check := accessx.Check{
				SkipPolicy:  accessx.SkipAuthz,
				AuditAction: "iam.organization.create",
			}
			return check, true, nil
		}

		// 3. Delegate to generated access resolvers.
		resolvers := []mwaccess.Resolver{
			v1.IAMAuthServiceKernelAccessResolver,
			v1.IAMDirectoryServiceKernelAccessResolver,
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
}

// resolveAuditAction returns a default audit action for a given operation.
func resolveAuditAction(operation string) string {
	// Extract the last segment of the operation name as the audit action.
	if idx := strings.LastIndex(operation, "/"); idx >= 0 {
		return "iam." + strings.ToLower(operation[idx+1:])
	}
	if idx := strings.LastIndex(operation, "."); idx >= 0 {
		return "iam." + strings.ToLower(operation[idx+1:])
	}
	return "iam." + strings.ToLower(operation)
}

func isGetMeOperation(operation string) bool {
	switch operation {
	case "GetMe", "iam.v1.IAMAuthService/GetMe", "/iam.v1.IAMAuthService/GetMe":
		return true
	}
	return false
}

func isCreateOrganizationOperation(operation string) bool {
	switch operation {
	case "CreateOrganization", "iam.project.v1.ProjectService/CreateOrganization", "/iam.project.v1.ProjectService/CreateOrganization":
		return true
	}
	// HTTP gateway routes use URL path templates as the operation.
	if operation == "/v1/iam/control-plane/orgs" {
		return true
	}
	return false
}
