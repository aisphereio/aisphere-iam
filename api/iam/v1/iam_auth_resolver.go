package v1

import (
	"context"

	"github.com/aisphereio/kernel/accessx"
	accessv1 "github.com/aisphereio/kernel/api/aisphere/access/v1"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/requestx"
)

var iamAuthServiceRules = authz.Rules{
	"/iam.v1.IAMAuthService/RefreshToken": {
		Service:    "iam.v1.IAMAuthService",
		Method:     "RefreshToken",
		FullMethod: "/iam.v1.IAMAuthService/RefreshToken",
		Action:     "refresh",
		Resource:   "iam:session",
		Audience:   "iam-service",
		Mode:       authz.RuleModeCheckOnly,
		AuditEvent: "iam.refresh_token",
		AuditRisk:  "medium",
	},
	"/iam.v1.IAMAuthService/VerifyToken": {
		Service:    "iam.v1.IAMAuthService",
		Method:     "VerifyToken",
		FullMethod: "/iam.v1.IAMAuthService/VerifyToken",
		Action:     "verify",
		Resource:   "iam:token",
		Audience:   "iam-service",
		Mode:       authz.RuleModeCheckOnly,
		AuditEvent: "iam.verify_token",
		AuditRisk:  "medium",
	},
	"/iam.v1.IAMAuthService/RevokeToken": {
		Service:    "iam.v1.IAMAuthService",
		Method:     "RevokeToken",
		FullMethod: "/iam.v1.IAMAuthService/RevokeToken",
		Action:     "revoke",
		Resource:   "iam:session",
		Audience:   "iam-service",
		Mode:       authz.RuleModeCheckOnly,
		AuditEvent: "iam.revoke_token",
		AuditRisk:  "medium",
	},
	"/iam.v1.IAMAuthService/GetMe": {
		Service:    "iam.v1.IAMAuthService",
		Method:     "GetMe",
		FullMethod: "/iam.v1.IAMAuthService/GetMe",
		Action:     "read",
		Resource:   "iam:user:self",
		Audience:   "iam-service",
		Mode:       authz.RuleModeCheckOnly,
		AuditEvent: "iam.get_me",
		AuditRisk:  "low",
	},
}

func IAMAuthServiceRequestInfoResolver(ctx context.Context, operation string, req any) (requestx.Info, bool, error) {
	_ = ctx
	_ = req
	normalized := normalizeIAMAuthOperation(operation)
	switch normalized {
	case "/iam.v1.IAMAuthService/BuildLoginURL":
		return requestx.Info{
			Service:   "iam.v1.IAMAuthService",
			Method:    "BuildLoginURL",
			Operation: normalized,
			Exposure:  accessv1.Exposure_PUBLIC,
			Labels:    map[string]string{"audit_event": "iam.login_url", "audit_risk": "low"},
		}.Normalize(), true, nil
	case "/iam.v1.IAMAuthService/ExchangeCode":
		return requestx.Info{
			Service:   "iam.v1.IAMAuthService",
			Method:    "ExchangeCode",
			Operation: normalized,
			Exposure:  accessv1.Exposure_PUBLIC,
			Labels:    map[string]string{"audit_event": "iam.exchange_code", "audit_risk": "medium"},
		}.Normalize(), true, nil
	}
	rule, ok := iamAuthServiceRules[normalized]
	if !ok {
		return requestx.Info{}, false, nil
	}
	exposure := accessv1.Exposure_AUTHENTICATED
	if rule.Method == "VerifyToken" {
		exposure = accessv1.Exposure_INTERNAL
	}
	info := requestx.Info{
		Service:       rule.Service,
		Method:        rule.Method,
		Operation:     rule.FullMethod,
		Exposure:      exposure,
		Action:        rule.Action,
		Resource:      rule.Resource,
		TargetService: rule.Audience,
		Labels:        map[string]string{"authz_mode": string(rule.Mode), "audit_event": rule.AuditEvent, "audit_risk": rule.AuditRisk},
	}
	return info.Normalize(), true, nil
}

func IAMAuthServiceAccessResolver(ctx context.Context, operation string, req any) (accessx.Check, bool, error) {
	normalized := normalizeIAMAuthOperation(operation)
	if normalized == "/iam.v1.IAMAuthService/BuildLoginURL" || normalized == "/iam.v1.IAMAuthService/ExchangeCode" {
		return accessx.Check{}, false, nil
	}
	rule, ok := iamAuthServiceRules[normalized]
	if !ok {
		return accessx.Check{}, false, nil
	}
	resource, err := (authz.RuleResolver{}).ResolveResource(rule, req)
	if err != nil {
		return accessx.Check{}, true, err
	}
	_ = ctx
	return accessx.Check{
		Permission:  rule.Action,
		Resource:    resource,
		AuditAction: rule.AuditEvent,
		Metadata:    map[string]any{"authz_rule": rule.FullMethod, "authz_mode": string(rule.Mode)},
	}, true, nil
}

func normalizeIAMAuthOperation(operation string) string {
	switch operation {
	case "BuildLoginURL", "iam.v1.IAMAuthService/BuildLoginURL":
		return "/iam.v1.IAMAuthService/BuildLoginURL"
	case "ExchangeCode", "iam.v1.IAMAuthService/ExchangeCode":
		return "/iam.v1.IAMAuthService/ExchangeCode"
	case "RefreshToken", "iam.v1.IAMAuthService/RefreshToken":
		return "/iam.v1.IAMAuthService/RefreshToken"
	case "VerifyToken", "iam.v1.IAMAuthService/VerifyToken":
		return "/iam.v1.IAMAuthService/VerifyToken"
	case "RevokeToken", "iam.v1.IAMAuthService/RevokeToken":
		return "/iam.v1.IAMAuthService/RevokeToken"
	case "GetMe", "iam.v1.IAMAuthService/GetMe":
		return "/iam.v1.IAMAuthService/GetMe"
	default:
		return operation
	}
}
