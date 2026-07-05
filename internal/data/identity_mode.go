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
		return readOnlyIdentityAdmin{next: next}, nil
	}
	return next, nil
}

type readOnlyIdentityAdmin struct {
	next authn.IdentityAdmin
}

func (a readOnlyIdentityAdmin) ExchangeCode(ctx context.Context, req authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	return a.next.ExchangeCode(ctx, req)
}

func (a readOnlyIdentityAdmin) RefreshToken(ctx context.Context, req authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return a.next.RefreshToken(ctx, req)
}

func (a readOnlyIdentityAdmin) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	return a.next.VerifyToken(ctx, req)
}

func (a readOnlyIdentityAdmin) RevokeToken(ctx context.Context, req authn.RevokeTokenRequest) error {
	return a.next.RevokeToken(ctx, req)
}

func (a readOnlyIdentityAdmin) GetUser(ctx context.Context, orgID, userID string) (authn.User, error) {
	return a.next.GetUser(ctx, orgID, userID)
}

func (a readOnlyIdentityAdmin) FindUsers(ctx context.Context, filter authn.UserFilter) ([]authn.User, error) {
	return a.next.FindUsers(ctx, filter)
}

func (a readOnlyIdentityAdmin) CreateUser(ctx context.Context, req authn.CreateUserRequest) (authn.User, error) {
	return authn.User{}, readOnlyIdentityError("CreateUser")
}

func (a readOnlyIdentityAdmin) UpdateUser(ctx context.Context, req authn.UpdateUserRequest) (authn.User, error) {
	return authn.User{}, readOnlyIdentityError("UpdateUser")
}

func (a readOnlyIdentityAdmin) DeleteUser(ctx context.Context, req authn.DeleteUserRequest) error {
	return readOnlyIdentityError("DeleteUser")
}

func (a readOnlyIdentityAdmin) UpsertUser(ctx context.Context, user authn.User) (authn.User, error) {
	return authn.User{}, readOnlyIdentityError("UpsertUser")
}

func (a readOnlyIdentityAdmin) DisableUser(ctx context.Context, orgID, userID string) error {
	return readOnlyIdentityError("DisableUser")
}

func (a readOnlyIdentityAdmin) CreateOrganization(ctx context.Context, req authn.CreateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, readOnlyIdentityError("CreateOrganization")
}

func (a readOnlyIdentityAdmin) GetOrganization(ctx context.Context, orgID string) (authn.Organization, error) {
	return a.next.GetOrganization(ctx, orgID)
}

func (a readOnlyIdentityAdmin) UpdateOrganization(ctx context.Context, req authn.UpdateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, readOnlyIdentityError("UpdateOrganization")
}

func (a readOnlyIdentityAdmin) DeleteOrganization(ctx context.Context, req authn.DeleteOrganizationRequest) error {
	return readOnlyIdentityError("DeleteOrganization")
}

func (a readOnlyIdentityAdmin) CreateApplication(ctx context.Context, req authn.CreateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, readOnlyIdentityError("CreateApplication")
}

func (a readOnlyIdentityAdmin) GetApplication(ctx context.Context, orgID, appID string) (authn.Application, error) {
	return a.next.GetApplication(ctx, orgID, appID)
}

func (a readOnlyIdentityAdmin) UpdateApplication(ctx context.Context, req authn.UpdateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, readOnlyIdentityError("UpdateApplication")
}

func (a readOnlyIdentityAdmin) DeleteApplication(ctx context.Context, req authn.DeleteApplicationRequest) error {
	return readOnlyIdentityError("DeleteApplication")
}

func (a readOnlyIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	return authn.Group{}, readOnlyIdentityError("CreateGroup")
}

func (a readOnlyIdentityAdmin) GetGroup(ctx context.Context, orgID, groupID string) (authn.Group, error) {
	return a.next.GetGroup(ctx, orgID, groupID)
}

func (a readOnlyIdentityAdmin) ListGroups(ctx context.Context, filter authn.GroupFilter) ([]authn.Group, error) {
	return a.next.ListGroups(ctx, filter)
}

func (a readOnlyIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	return authn.Group{}, readOnlyIdentityError("UpdateGroup")
}

func (a readOnlyIdentityAdmin) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	return readOnlyIdentityError("DeleteGroup")
}

func (a readOnlyIdentityAdmin) AssignUserToGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return readOnlyIdentityError("AssignUserToGroup")
}

func (a readOnlyIdentityAdmin) RemoveUserFromGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return readOnlyIdentityError("RemoveUserFromGroup")
}

func readOnlyIdentityError(operation string) error {
	return authn.ErrIdentityBackendFailed("identity directory is read-only in external_oidc mode: "+operation, nil)
}

var _ authn.IdentityAdmin = readOnlyIdentityAdmin{}
