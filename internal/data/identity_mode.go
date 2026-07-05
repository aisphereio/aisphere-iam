package data

import (
	"context"
	"fmt"
	"strings"

	"github.com/aisphereio/kernel/authn"
)

const (
	IdentityModeCasdoorLocal = "casdoor_local"
	IdentityModeExternalOIDC = "external_oidc"
)

func identityMode(mode string) (string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "", IdentityModeCasdoorLocal:
		return IdentityModeCasdoorLocal, nil
	case IdentityModeExternalOIDC:
		return IdentityModeExternalOIDC, nil
	default:
		return "", fmt.Errorf("unsupported authn identity_mode: %s", mode)
	}
}

func identityForMode(mode string, next authn.IdentityAdmin) (authn.IdentityAdmin, error) {
	resolved, err := identityMode(mode)
	if err != nil {
		return nil, err
	}
	if next == nil {
		return nil, nil
	}
	if resolved == IdentityModeExternalOIDC {
		return externalOIDCIdentityAdmin{next: next}, nil
	}
	return next, nil
}

// externalOIDCIdentityAdmin protects the upstream user/org directory while still
// allowing Aisphere-owned application groups and group membership to be managed.
//
// In external OIDC mode, users and identity organizations come from an upstream
// identity source and are therefore read-only from IAM's perspective. Groups are
// different: IAM uses Casdoor groups as an application-layer authorization
// construct, including multi-level groups and user-to-group binding for local
// access projection. Those group operations intentionally remain writable.
type externalOIDCIdentityAdmin struct {
	next authn.IdentityAdmin
}

func (a externalOIDCIdentityAdmin) ExchangeCode(ctx context.Context, req authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	return a.next.ExchangeCode(ctx, req)
}

func (a externalOIDCIdentityAdmin) RefreshToken(ctx context.Context, req authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return a.next.RefreshToken(ctx, req)
}

func (a externalOIDCIdentityAdmin) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	return a.next.VerifyToken(ctx, req)
}

func (a externalOIDCIdentityAdmin) RevokeToken(ctx context.Context, req authn.RevokeTokenRequest) error {
	return a.next.RevokeToken(ctx, req)
}

func (a externalOIDCIdentityAdmin) GetUser(ctx context.Context, orgID, userID string) (authn.User, error) {
	return a.next.GetUser(ctx, orgID, userID)
}

func (a externalOIDCIdentityAdmin) FindUsers(ctx context.Context, filter authn.UserFilter) ([]authn.User, error) {
	return a.next.FindUsers(ctx, filter)
}

func (a externalOIDCIdentityAdmin) CreateUser(ctx context.Context, req authn.CreateUserRequest) (authn.User, error) {
	return authn.User{}, externalDirectoryReadOnlyError("CreateUser")
}

func (a externalOIDCIdentityAdmin) UpdateUser(ctx context.Context, req authn.UpdateUserRequest) (authn.User, error) {
	return authn.User{}, externalDirectoryReadOnlyError("UpdateUser")
}

func (a externalOIDCIdentityAdmin) DeleteUser(ctx context.Context, req authn.DeleteUserRequest) error {
	return externalDirectoryReadOnlyError("DeleteUser")
}

func (a externalOIDCIdentityAdmin) UpsertUser(ctx context.Context, user authn.User) (authn.User, error) {
	return authn.User{}, externalDirectoryReadOnlyError("UpsertUser")
}

func (a externalOIDCIdentityAdmin) DisableUser(ctx context.Context, orgID, userID string) error {
	return externalDirectoryReadOnlyError("DisableUser")
}

func (a externalOIDCIdentityAdmin) CreateOrganization(ctx context.Context, req authn.CreateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, externalDirectoryReadOnlyError("CreateOrganization")
}

func (a externalOIDCIdentityAdmin) GetOrganization(ctx context.Context, orgID string) (authn.Organization, error) {
	return a.next.GetOrganization(ctx, orgID)
}

func (a externalOIDCIdentityAdmin) UpdateOrganization(ctx context.Context, req authn.UpdateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, externalDirectoryReadOnlyError("UpdateOrganization")
}

func (a externalOIDCIdentityAdmin) DeleteOrganization(ctx context.Context, req authn.DeleteOrganizationRequest) error {
	return externalDirectoryReadOnlyError("DeleteOrganization")
}

func (a externalOIDCIdentityAdmin) CreateApplication(ctx context.Context, req authn.CreateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, externalDirectoryReadOnlyError("CreateApplication")
}

func (a externalOIDCIdentityAdmin) GetApplication(ctx context.Context, orgID, appID string) (authn.Application, error) {
	return a.next.GetApplication(ctx, orgID, appID)
}

func (a externalOIDCIdentityAdmin) UpdateApplication(ctx context.Context, req authn.UpdateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, externalDirectoryReadOnlyError("UpdateApplication")
}

func (a externalOIDCIdentityAdmin) DeleteApplication(ctx context.Context, req authn.DeleteApplicationRequest) error {
	return externalDirectoryReadOnlyError("DeleteApplication")
}

func (a externalOIDCIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	return a.next.CreateGroup(ctx, req)
}

func (a externalOIDCIdentityAdmin) GetGroup(ctx context.Context, orgID, groupID string) (authn.Group, error) {
	return a.next.GetGroup(ctx, orgID, groupID)
}

func (a externalOIDCIdentityAdmin) ListGroups(ctx context.Context, filter authn.GroupFilter) ([]authn.Group, error) {
	return a.next.ListGroups(ctx, filter)
}

func (a externalOIDCIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	return a.next.UpdateGroup(ctx, req)
}

func (a externalOIDCIdentityAdmin) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	return a.next.DeleteGroup(ctx, req)
}

func (a externalOIDCIdentityAdmin) AssignUserToGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return a.next.AssignUserToGroup(ctx, req)
}

func (a externalOIDCIdentityAdmin) RemoveUserFromGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return a.next.RemoveUserFromGroup(ctx, req)
}

func externalDirectoryReadOnlyError(operation string) error {
	return authn.ErrIdentityBackendFailed("identity user/org directory is read-only in external_oidc mode: "+operation, nil)
}

var _ authn.IdentityAdmin = externalOIDCIdentityAdmin{}
