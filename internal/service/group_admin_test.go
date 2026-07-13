package service

import (
	"context"
	"testing"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

func TestIAMGroupAdminServiceCreateUpdateDelete(t *testing.T) {
	deps := IAMDeps{
		Identity: newFakeIdentityAdmin(),
		Authz:    authz.NewMemoryRelationshipStore(),
	}
	svc := NewIAMGroupAdminService(deps)
	ctx := context.Background()

	// Create group
	created, err := svc.CreateGroup(ctx, &v1.CreateGroupRequest{
		OrgID: "aisphere",
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
		OrgID:   "aisphere",
		GroupID: "dev-team",
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
		OrgID: "aisphere", GroupID: "dev-team", UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("AssignUserToGroup: %v", err)
	}

	// Remove user from group
	_, err = svc.RemoveUserFromGroup(ctx, &v1.RemoveUserFromGroupRequest{
		OrgID: "aisphere", GroupID: "dev-team", UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("RemoveUserFromGroup: %v", err)
	}

	// Delete group
	_, err = svc.DeleteGroup(ctx, &v1.DeleteGroupRequest{
		OrgID: "aisphere", GroupID: "dev-team",
	})
	if err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
}

// fakeIdentityAdmin implements authn.IdentityAdmin for testing
type fakeIdentityAdmin struct {
	authn.UnimplementedIdentityAdmin
	groups map[string]authn.Group
	users  map[string]authn.User
}

func newFakeIdentityAdmin() *fakeIdentityAdmin {
	return &fakeIdentityAdmin{
		groups: make(map[string]authn.Group),
		users:  make(map[string]authn.User),
	}
}

func (f *fakeIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	group := req.Group
	if group.ID == "" {
		group.ID = "group-" + group.Name
	}
	f.groups[group.ID] = group
	return group, nil
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