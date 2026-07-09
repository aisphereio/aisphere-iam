package service

import (
	"context"
	"strings"

	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

// DirectoryProjectionOps is the service-layer implementation behind the
// proto-first IAMDirectoryProjectionService contract.
//
// Until api/iam/v1/iam.proto is regenerated, HTTP compatibility shims call this
// type directly. After running `make api && make deploy`, generated handlers
// should delegate to these methods and the compatibility shims can be removed.
type DirectoryProjectionOps struct {
	Identity   authn.IdentityAdmin
	Authz      authz.AdminProvider
	Projection *data.IdentityProjectionDispatcher
}

type RetryDirectoryProjectionResult struct {
	Processed int `json:"processed"`
}

type ReconcileDirectoryProjectionResult struct {
	Status        string `json:"status"`
	Relationships int    `json:"relationships"`
}

type CheckDirectoryProjectionDriftResult struct {
	Desired              int                  `json:"desired"`
	Missing              int                  `json:"missing"`
	MissingRelationships []string             `json:"missing_relationships"`
	MissingObjects       []authz.Relationship `json:"missing_relationship_objects"`
}

func NewDirectoryProjectionOps(resources interface {
	GetIdentityProjection() *data.IdentityProjectionDispatcher
	GetIdentity() authn.IdentityAdmin
	GetAuthzAdmin() authz.AdminProvider
}) *DirectoryProjectionOps {
	if resources == nil {
		return nil
	}
	return &DirectoryProjectionOps{Identity: resources.GetIdentity(), Authz: resources.GetAuthzAdmin(), Projection: resources.GetIdentityProjection()}
}

func NewDirectoryProjectionOpsFromDeps(identity authn.IdentityAdmin, admin authz.AdminProvider, projection *data.IdentityProjectionDispatcher) *DirectoryProjectionOps {
	return &DirectoryProjectionOps{Identity: identity, Authz: admin, Projection: projection}
}

func (s *DirectoryProjectionOps) Retry(ctx context.Context, limit int) (RetryDirectoryProjectionResult, error) {
	if s == nil || s.Projection == nil {
		return RetryDirectoryProjectionResult{}, authz.ErrBackendFailed("directory projection dispatcher is not configured", nil)
	}
	processed, err := s.Projection.RetryOnce(ctx, limit)
	return RetryDirectoryProjectionResult{Processed: processed}, err
}

func (s *DirectoryProjectionOps) Reconcile(ctx context.Context, orgID string, source string) (ReconcileDirectoryProjectionResult, error) {
	if s == nil || s.Identity == nil || s.Projection == nil {
		return ReconcileDirectoryProjectionResult{}, authz.ErrBackendFailed("directory projection service is not configured", nil)
	}
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return ReconcileDirectoryProjectionResult{}, authn.ErrInvalidTokenRequest("org_id is required")
	}
	rels, err := data.BuildDirectoryProjectionRelationships(ctx, s.Identity, orgID)
	if err != nil {
		return ReconcileDirectoryProjectionResult{}, err
	}
	if err := s.Projection.Dispatch(ctx, firstNonEmptyProjectionSource(source, "reconcile"), "zone", orgID, data.IdentityAuthZProjectionPayload{Operation: "write", Relationships: rels}); err != nil {
		return ReconcileDirectoryProjectionResult{}, err
	}
	return ReconcileDirectoryProjectionResult{Status: "submitted", Relationships: len(rels)}, nil
}

func (s *DirectoryProjectionOps) Drift(ctx context.Context, orgID string) (CheckDirectoryProjectionDriftResult, error) {
	if s == nil || s.Identity == nil || s.Authz == nil {
		return CheckDirectoryProjectionDriftResult{}, authz.ErrBackendFailed("directory projection service is not configured", nil)
	}
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return CheckDirectoryProjectionDriftResult{}, authn.ErrInvalidTokenRequest("org_id is required")
	}
	desired, err := data.BuildDirectoryProjectionRelationships(ctx, s.Identity, orgID)
	if err != nil {
		return CheckDirectoryProjectionDriftResult{}, err
	}
	missing, err := data.DetectDirectoryProjectionDrift(ctx, s.Authz, desired)
	if err != nil {
		return CheckDirectoryProjectionDriftResult{}, err
	}
	return CheckDirectoryProjectionDriftResult{Desired: len(desired), Missing: len(missing), MissingRelationships: relationshipStrings(missing), MissingObjects: missing}, nil
}

func firstNonEmptyProjectionSource(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func relationshipStrings(rels []authz.Relationship) []string {
	out := make([]string, 0, len(rels))
	for _, rel := range rels {
		s := rel.Resource.Type + ":" + rel.Resource.ID + "#" + rel.Relation + "@" + rel.Subject.Type + ":" + rel.Subject.ID
		if rel.Subject.Relation != "" {
			s += "#" + rel.Subject.Relation
		}
		out = append(out, s)
	}
	return out
}
