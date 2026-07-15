// Package scope provides OrgScope and ScopeGuard for validating organization
// scope in IAM service methods.
//
// Usage:
//
//	guard := scope.NewGuard(authorizer)
//	scp, err := guard.Validate(ctx, pathOrgID, principal)
//	if err != nil {
//	    return nil, err
//	}
//	if err := guard.RequirePermission(ctx, scp, subject, resource, permission); err != nil {
//	    return nil, err
//	}
package scope

import (
	"context"
	"strings"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

// OrgScope represents a validated organization scope. It is created by
// ScopeGuard.Validate and ensures that the path org_id matches the
// authenticated principal's org_id.
type OrgScope struct {
	OrgID string
}

// ScopeGuard validates organization scope and enforces authorization.
type ScopeGuard struct {
	authorizer authz.Authorizer
}

// NewScopeGuard creates a new ScopeGuard with the given authorizer.
func NewScopeGuard(authorizer authz.Authorizer) *ScopeGuard {
	return &ScopeGuard{authorizer: authorizer}
}

// Validate checks that the path org_id matches the authenticated principal's
// org_id. If pathOrgID is empty, the principal's org_id is used. Returns
// the validated OrgScope.
func (g *ScopeGuard) Validate(ctx context.Context, pathOrgID string, principal authn.Principal) (*OrgScope, error) {
	principalOrgID := strings.TrimSpace(principal.OrgID)
	if principalOrgID == "" {
		return nil, authn.ErrMissingCredential("principal org_id is required")
	}
	if pathOrgID != "" && !strings.EqualFold(pathOrgID, principalOrgID) {
		return nil, authz.ErrPermissionDenied("org_id mismatch: path org_id does not match principal org_id")
	}
	return &OrgScope{OrgID: principalOrgID}, nil
}

// PrincipalFromContext extracts the authenticated principal from context.
// Returns an error if the principal is missing or unauthenticated.
func PrincipalFromContext(ctx context.Context) (authn.Principal, error) {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || !principal.IsAuthenticated() {
		return authn.Principal{}, authn.ErrMissingCredential("kernel principal is required")
	}
	return principal.Normalize(), nil
}

// SubjectRefFromPrincipal converts a principal to an authz SubjectRef.
func SubjectRefFromPrincipal(principal authn.Principal) authz.SubjectRef {
	subjectType := strings.TrimSpace(principal.SubjectType)
	if subjectType == "" {
		subjectType = authz.SubjectTypeUser
	}
	return authz.SubjectRef{Type: subjectType, ID: strings.TrimSpace(principal.SubjectID)}
}

// RequirePermission checks that the subject has the given permission on the
// given resource within the org scope.
func (g *ScopeGuard) RequirePermission(ctx context.Context, scope *OrgScope, subject authz.SubjectRef, resource authz.ObjectRef, permission string) error {
	if g.authorizer == nil {
		return authz.ErrBackendFailed("authorization provider is not configured", nil)
	}
	decision, err := g.authorizer.Check(ctx, authz.CheckRequest{
		Subject:    subject,
		Resource:   resource,
		Permission: permission,
		OrgID:      scope.OrgID,
	})
	if err != nil {
		return err
	}
	if !decision.IsAllowed() {
		return authz.ErrPermissionDenied("permission denied: " + resource.String() + "#" + permission)
	}
	return nil
}