package service

import (
	"context"
	"strings"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"google.golang.org/protobuf/types/known/emptypb"
)

// IAMGroupAdminService implements the canonical Group management surface.
//
// This consolidates Group CRUD and membership operations that were previously
// split across IAMDirectoryService and IAMIdentityAdminService.
//
// AuthN/AuthZ is enforced by iamServerMiddlewares before these methods run.
// Identity-mode behavior is enforced by the configured authn.IdentityAdmin adapter:
//
//   - casdoor_local: group writes are allowed after AuthZ.
//   - external_oidc: group writes remain allowed (Aisphere-owned groups).
//
// Group writes are projected into AuthZ by data.BindIdentityAuthZ.
//
// org_id is extracted from the URL path (not the request body) and validated
// against the authenticated principal's org_id to prevent cross-org access.
type IAMGroupAdminService struct {
	v1.UnimplementedIAMGroupAdminServiceServer
	deps IAMDeps
}

func NewIAMGroupAdminService(deps IAMDeps) *IAMGroupAdminService {
	return &IAMGroupAdminService{deps: deps}
}

// orgIDFromPrincipal validates the path org_id against the authenticated
// principal's org_id. The path org_id is bound by the proto-generated HTTP
// handler from the URL; we cross-check it against the principal to prevent
// body-override attacks. Returns the validated org_id.
func (s *IAMGroupAdminService) orgIDFromPrincipal(ctx context.Context, pathOrgID string) (string, error) {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || !principal.IsAuthenticated() {
		return "", authn.ErrMissingCredential("kernel principal is required")
	}
	principalOrgID := strings.TrimSpace(principal.OrgID)
	if principalOrgID == "" {
		return "", authn.ErrMissingCredential("kernel principal org_id is required")
	}
	if pathOrgID != "" && !strings.EqualFold(pathOrgID, principalOrgID) {
		return "", authz.ErrPermissionDenied("org_id mismatch: path org_id does not match principal org_id")
	}
	return principalOrgID, nil
}

func (s *IAMGroupAdminService) CreateGroup(ctx context.Context, req *v1.CreateGroupRequest) (*v1.Group, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	orgID, err := s.orgIDFromPrincipal(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}
	group := groupFromProto(req.GetGroup())
	group.OrgID = orgID
	created, err := s.deps.Identity.CreateGroup(ctx, authn.CreateGroupRequest{
		Group:          group,
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, err
	}
	return groupToProto(created), nil
}

func (s *IAMGroupAdminService) UpdateGroup(ctx context.Context, req *v1.UpdateGroupRequest) (*v1.Group, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	orgID, err := s.orgIDFromPrincipal(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}
	group := groupFromProto(req.GetGroup())
	group.ID = firstNonEmptyString(group.ID, req.GetGroupId())
	group.OrgID = orgID
	updated, err := s.deps.Identity.UpdateGroup(ctx, authn.UpdateGroupRequest{Group: group})
	if err != nil {
		return nil, err
	}
	return groupToProto(updated), nil
}

func (s *IAMGroupAdminService) DeleteGroup(ctx context.Context, req *v1.DeleteGroupRequest) (*emptypb.Empty, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	orgID, err := s.orgIDFromPrincipal(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}
	if err := s.deps.Identity.DeleteGroup(ctx, authn.DeleteGroupRequest{
		OrgID:     orgID,
		GroupID:   req.GetGroupId(),
		Recursive: req.GetRecursive(),
	}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *IAMGroupAdminService) AssignUserToGroup(ctx context.Context, req *v1.AssignUserToGroupRequest) (*emptypb.Empty, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	orgID, err := s.orgIDFromPrincipal(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}
	if err := s.deps.Identity.AssignUserToGroup(ctx, authn.AssignUserToGroupRequest{
		OrgID:   orgID,
		GroupID: req.GetGroupId(),
		UserID:  req.GetUserId(),
	}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *IAMGroupAdminService) RemoveUserFromGroup(ctx context.Context, req *v1.RemoveUserFromGroupRequest) (*emptypb.Empty, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	orgID, err := s.orgIDFromPrincipal(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}
	if err := s.deps.Identity.RemoveUserFromGroup(ctx, authn.AssignUserToGroupRequest{
		OrgID:   orgID,
		GroupID: req.GetGroupId(),
		UserID:  req.GetUserId(),
	}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}