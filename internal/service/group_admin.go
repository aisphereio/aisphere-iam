package service

import (
	"context"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
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
type IAMGroupAdminService struct {
	v1.UnimplementedIAMGroupAdminServiceServer
	deps IAMDeps
}

func NewIAMGroupAdminService(deps IAMDeps) *IAMGroupAdminService {
	return &IAMGroupAdminService{deps: deps}
}

func (s *IAMGroupAdminService) CreateGroup(ctx context.Context, req *v1.CreateGroupRequest) (*v1.Group, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	group := groupFromProto(req.GetGroup())
	group.OrgID = firstNonEmptyString(group.OrgID, req.GetOrgId())
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
	group := groupFromProto(req.GetGroup())
	group.ID = firstNonEmptyString(group.ID, req.GetGroupId())
	group.OrgID = firstNonEmptyString(group.OrgID, req.GetOrgId())
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
	if err := s.deps.Identity.DeleteGroup(ctx, authn.DeleteGroupRequest{
		OrgID:     req.GetOrgId(),
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
	if err := s.deps.Identity.AssignUserToGroup(ctx, authn.AssignUserToGroupRequest{
		OrgID:   req.GetOrgId(),
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
	if err := s.deps.Identity.RemoveUserFromGroup(ctx, authn.AssignUserToGroupRequest{
		OrgID:   req.GetOrgId(),
		GroupID: req.GetGroupId(),
		UserID:  req.GetUserId(),
	}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}