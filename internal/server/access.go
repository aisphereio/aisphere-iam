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
	// IAM now runs behind Envoy Gateway OIDC. Gateway verifies the identity and
	// injects x-aisphere-* claim headers; kernel v0.4.0 restores Principal from
	// those headers. Do not require the old Gateway-to-backend shared header for
	// this OIDC-only path.
	internalCall := cfg.InternalCall
	if strings.EqualFold(cfg.Authn.Mode, securityx.AuthnModeGatewayTrusted) {
		internalCall.Enabled = false
		internalCall.Token = ""
	}

	runtime, err := securityx.NewRuntime(context.Background(), securityx.Config{
		Authn: securityx.AuthnBoundaryConfig{
			Enabled:        cfg.Authn.Enabled,
			Mode:           cfg.Authn.Mode,
			Provider:       cfg.Authn.Provider,
			OIDC:           cfg.Authn.OIDC,
			CacheTTL:       cfg.Authn.CacheTTL,
			InternalCall:   internalCall,
			AllowAnonymous: true,
		},
		InternalCall: internalCall,
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
		case "healthz", "readyz", "metrics", "iam.v1.IAMAuthService/ExternalAuthorize":
			return accessx.SkipAll
		}
		if op == "ExternalAuthorize" {
			return accessx.SkipAll
		}
		if isManualGroupManagementOperation(op) {
			// Manual group write routes are not generated from proto yet. Keep authn
			// and audit through accessx, but skip resource-level SpiceDB checks until
			// the group-management contract is promoted into generated API metadata.
			return accessx.SkipAuthz
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

func isManualGroupManagementOperation(op string) bool {
	op = strings.TrimPrefix(strings.TrimSpace(op), "/")
	return op == "v1/iam/orgs/{org_id}/groups" || op == "v1/iam/orgs/{org_id}/groups/{group_id}"
}
