package service

import (
	"context"
	"testing"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
)

func TestIAMGroupAdminServiceCreateUpdateDelete(t *testing.T) {
	deps := IAMDeps{
		Identity: newFakeIdentityAdmin(),
	}
	svc := NewIAMGroupAdminService(deps)
	ctx := context.Background()

	// Create group
	created, err := svc.CreateGroup(ctx, &v1.CreateGroupRequest{
		OrgId: "aisphere",
		Group: &v1.Group{Id: "dev-team", Name: "Dev Team", DisplayName: "Development Team"},
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if created.GetId() != "dev-team" {
		t.Fatalf("unexpected group id: %s", created.GetId())
	}

	// Update group
	updated, err := svc.UpdateGroup(ctx, &v1.UpdateGroupRequest{
		OrgId:   "aisphere",
		GroupId: "dev-team",
		Group:   &v1.Group{Id: "dev-team", Name: "Dev Team", DisplayName: "Development Team v2"},
	})
	if err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	if updated.GetDisplayName() != "Development Team v2" {
		t.Fatalf("unexpected display name: %s", updated.GetDisplayName())
	}

	// Assign user to group
	_, err = svc.AssignUserToGroup(ctx, &v1.AssignUserToGroupRequest{
		OrgId: "aisphere", GroupId: "dev-team", UserId: "user-1",
	})
	if err != nil {
		t.Fatalf("AssignUserToGroup: %v", err)
	}

	// Remove user from group
	_, err = svc.RemoveUserFromGroup(ctx, &v1.RemoveUserFromGroupRequest{
		OrgId: "aisphere", GroupId: "dev-team", UserId: "user-1",
	})
	if err != nil {
		t.Fatalf("RemoveUserFromGroup: %v", err)
	}

	// Delete group
	_, err = svc.DeleteGroup(ctx, &v1.DeleteGroupRequest{
		OrgId: "aisphere", GroupId: "dev-team",
	})
	if err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
}

// fakeIdentityAdmin implements authn.IdentityAdmin for testing
type fakeIdentityAdmin struct {
	groups map[string]authn.Group
	users  map[string]authn.User
}

func newFakeIdentityAdmin() *fakeIdentityAdmin {
	return &fakeIdentityAdmin{
		groups: make(map[string]authn.Group),
		users:  make(map[string]authn.User),
	}
}

func (f *fakeIdentityAdmin) ExchangeCode(ctx context.Context, req authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	return authn.TokenSet{}, authn.Principal{}, nil
}
func (f *fakeIdentityAdmin) RefreshToken(ctx context.Context, req authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return authn.TokenSet{}, nil
}
func (f *fakeIdentityAdmin) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	return authn.Principal{}, nil
}
func (f *fakeIdentityAdmin) RevokeToken(ctx context.Context, req authn.RevokeTokenRequest) error {
	return nil
}
func (f *fakeIdentityAdmin) GetUser(ctx context.Context, orgID, userID string) (authn.User, error) {
	return authn.User{}, nil
}
func (f *fakeIdentityAdmin) FindUsers(ctx context.Context, filter authn.UserFilter) ([]authn.User, error) {
	return nil, nil
}
func (f *fakeIdentityAdmin) CreateUser(ctx context.Context, req authn.CreateUserRequest) (authn.User, error) {
	return authn.User{}, nil
}
func (f *fakeIdentityAdmin) UpdateUser(ctx context.Context, req authn.UpdateUserRequest) (authn.User, error) {
	return authn.User{}, nil
}
func (f *fakeIdentityAdmin) DeleteUser(ctx context.Context, req authn.DeleteUserRequest) error {
	return nil
}
func (f *fakeIdentityAdmin) UpsertUser(ctx context.Context, user authn.User) (authn.User, error) {
	return authn.User{}, nil
}
func (f *fakeIdentityAdmin) DisableUser(ctx context.Context, orgID, userID string) error {
	return nil
}
func (f *fakeIdentityAdmin) CreateOrganization(ctx context.Context, req authn.CreateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, nil
}
func (f *fakeIdentityAdmin) GetOrganization(ctx context.Context, orgID string) (authn.Organization, error) {
	return authn.Organization{}, nil
}
func (f *fakeIdentityAdmin) UpdateOrganization(ctx context.Context, req authn.UpdateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, nil
}
func (f *fakeIdentityAdmin) DeleteOrganization(ctx context.Context, req authn.DeleteOrganizationRequest) error {
	return nil
}
func (f *fakeIdentityAdmin) CreateApplication(ctx context.Context, req authn.CreateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, nil
}
func (f *fakeIdentityAdmin) GetApplication(ctx context.Context, orgID, appID string) (authn.Application, error) {
	return authn.Application{}, nil
}
func (f *fakeIdentityAdmin) UpdateApplication(ctx context.Context, req authn.UpdateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, nil
}
func (f *fakeIdentityAdmin) DeleteApplication(ctx context.Context, req authn.DeleteApplicationRequest) error {
	return nil
}
func (f *fakeIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	group := req.Group
	if group.ID == "" {
		group.ID = "group-" + group.Name
	}
	f.groups[group.ID] = group
	return group, nil
}
func (f *fakeIdentityAdmin) GetGroup(ctx context.Context, orgID, groupID string) (authn.Group, error) {
	return f.groups[groupID], nil
}
func (f *fakeIdentityAdmin) ListGroups(ctx context.Context, filter authn.GroupFilter) ([]authn.Group, error) {
	out := make([]authn.Group, 0, len(f.groups))
	for _, g := range f.groups {
		out = append(out, g)
	}
	return out, nil
}
func (f *fakeIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	f.groups[req.Group.ID] = req.Group
	return req.Group, nil
}
func (f *fakeIdentityAdmin) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	delete(f.groups, req.GroupID)
	return nil
}
func (f *fakeIdentityAdmin) AssignUserToGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return nil
}
func (f *fakeIdentityAdmin) RemoveUserFromGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return nil
}