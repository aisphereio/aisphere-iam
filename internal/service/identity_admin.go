package service

import (
	"context"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
	"google.golang.org/protobuf/types/known/emptypb"
)

// IAMIdentityAdminService implements the generated identity-management surface.
//
// AuthN/AuthZ is intentionally not duplicated here. The generated proto policy
// is enforced by iamServerMiddlewares before these methods run. Identity-mode
// behavior is enforced by the configured authn.IdentityAdmin adapter:
//
//   - casdoor_local: user writes are allowed after AuthZ.
//   - external_oidc: upstream user writes are rejected.
//
// Group CRUD and membership operations have been consolidated into
// IAMGroupAdminService (internal/service/group_admin.go).
type IAMIdentityAdminService struct {
	v1.UnimplementedIAMIdentityAdminServiceServer
	deps IAMDeps
}

func NewIAMIdentityAdminService(deps IAMDeps) *IAMIdentityAdminService {
	return &IAMIdentityAdminService{deps: deps}
}

func (s *IAMIdentityAdminService) CreateUser(ctx context.Context, req *v1.CreateUserRequest) (*v1.User, error) {
		if s.deps.Identity == nil {
			return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
		}
		user := userFromProto(req.GetUser())
		// org_id comes from the path (validated against Principal.org_id by middleware).
		// Body org_id is ignored to prevent privilege escalation.
		user.OrgID = req.GetOrgId()
		created, err := s.deps.Identity.CreateUser(ctx, authn.CreateUserRequest{
			User:           user,
			IdempotencyKey: req.GetIdempotencyKey(),
		})
		if err != nil {
			return nil, err
		}
		return userToProto(created), nil
	}

func (s *IAMIdentityAdminService) UpdateUser(ctx context.Context, req *v1.UpdateUserRequest) (*v1.User, error) {
		if s.deps.Identity == nil {
			return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
		}
		user := userFromProto(req.GetUser())
		user.ID = firstNonEmptyString(user.ID, req.GetUserId())
		// org_id must come from the path; body org_id is ignored.
		user.OrgID = req.GetOrgId()
	updated, err := s.deps.Identity.UpdateUser(ctx, authn.UpdateUserRequest{User: user})
	if err != nil {
		return nil, err
	}
	return userToProto(updated), nil
}

func (s *IAMIdentityAdminService) DisableUser(ctx context.Context, req *v1.DisableUserRequest) (*emptypb.Empty, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	if err := s.deps.Identity.DisableUser(ctx, req.GetOrgId(), req.GetUserId()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *IAMIdentityAdminService) DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*emptypb.Empty, error) {
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	if err := s.deps.Identity.DeleteUser(ctx, authn.DeleteUserRequest{
		OrgID:  req.GetOrgId(),
		UserID: req.GetUserId(),
		Hard:   req.GetHard(),
	}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func userFromProto(in *v1.User) authn.User {
	if in == nil {
		return authn.User{}
	}
	return authn.User{
		ID:          in.GetId(),
		ExternalID:  in.GetExternalId(),
		Provider:    in.GetProvider(),
		OrgID:       in.GetOrgId(),
		Username:    in.GetUsername(),
		DisplayName: in.GetDisplayName(),
		Email:       in.GetEmail(),
		Phone:       in.GetPhone(),
		Roles:       append([]string(nil), in.GetRoles()...),
		Groups:      append([]string(nil), in.GetGroups()...),
		Enabled:     in.GetEnabled(),
	}
}

func groupFromProto(in *v1.Group) authn.Group {
	if in == nil {
		return authn.Group{}
	}
	return authn.Group{
		ID:          in.GetId(),
		ExternalID:  in.GetExternalId(),
		OrgID:       in.GetOrgId(),
		ParentID:    in.GetParentId(),
		Name:        in.GetName(),
		DisplayName: in.GetDisplayName(),
		Type:        in.GetType(),
		Path:        in.GetPath(),
		Users:       append([]string(nil), in.GetUsers()...),
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
