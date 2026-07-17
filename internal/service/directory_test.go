package service

import (
	"context"
	"testing"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

func newDirectoryTestDeps() IAMDeps {
	store := authz.NewMemoryRelationshipStore()
	// Grant all needed permissions
	store.WriteRelationships(context.Background(),
		authz.Relationship{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "view_users", Subject: authz.SubjectRef{Type: "user", ID: "admin"}},
		authz.Relationship{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "view_zone", Subject: authz.SubjectRef{Type: "user", ID: "admin"}},
		authz.Relationship{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "view_groups", Subject: authz.SubjectRef{Type: "user", ID: "admin"}},
	)
	return IAMDeps{
		Identity: newFakeIdentityAdmin(),
		Authz:    memoryAuthzAdmin{MemoryRelationshipStore: store},
	}
}

func newDirectoryContext() context.Context {
	return authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID: "admin", SubjectType: "user", OrgID: "aisphere",
	})
}

func TestIAMDirectoryServiceGetUser(t *testing.T) {
	svc := NewIAMDirectoryService(newDirectoryTestDeps())
	user, err := svc.GetUser(newDirectoryContext(), &v1.GetUserRequest{OrgId: "aisphere", UserId: "admin"})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	t.Logf("User: %s", user.GetUsername())
}

func TestIAMDirectoryServiceListUsers(t *testing.T) {
	svc := NewIAMDirectoryService(newDirectoryTestDeps())
	users, err := svc.ListUsers(newDirectoryContext(), &v1.ListUsersRequest{OrgId: "aisphere"})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	t.Logf("Users: %d", len(users.GetUsers()))
}

func TestIAMDirectoryServiceGetOrganization(t *testing.T) {
	svc := NewIAMDirectoryService(newDirectoryTestDeps())
	org, err := svc.GetOrganization(newDirectoryContext(), &v1.GetOrganizationRequest{OrgId: "aisphere"})
	if err != nil {
		t.Fatalf("GetOrganization: %v", err)
	}
	t.Logf("Org: %s", org.GetName())
}

func TestIAMDirectoryServiceListGroups(t *testing.T) {
	svc := NewIAMDirectoryService(newDirectoryTestDeps())
	groups, err := svc.ListGroups(newDirectoryContext(), &v1.ListGroupsRequest{OrgId: "aisphere"})
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	t.Logf("Groups: %d", len(groups.GetGroups()))
}

func TestIAMDirectoryServiceListGroupsMatchesUserMembershipByExternalID(t *testing.T) {
	deps := newDirectoryTestDeps()
	deps.Identity = directoryAliasIdentity{}
	svc := NewIAMDirectoryService(deps)

	groups, err := svc.ListGroups(newDirectoryContext(), &v1.ListGroupsRequest{
		OrgId:  "aisphere",
		UserId: "user-1",
	})
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups.GetGroups()) != 1 {
		t.Fatalf("groups = %#v, want the stable group resolved through external_id", groups.GetGroups())
	}
	if got := groups.GetGroups()[0].GetId(); got != "grp_stable" {
		t.Fatalf("group id = %q, want grp_stable", got)
	}
}

func TestIAMDirectoryServiceListGroupsMatchesProviderGroupMembersWhenUserAliasIsStale(t *testing.T) {
	deps := newDirectoryTestDeps()
	deps.Identity = directoryStaleAliasIdentity{}
	svc := NewIAMDirectoryService(deps)

	groups, err := svc.ListGroups(newDirectoryContext(), &v1.ListGroupsRequest{
		OrgId:  "aisphere",
		UserId: "user-1",
	})
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups.GetGroups()) != 1 || groups.GetGroups()[0].GetId() != "grp_stable" {
		t.Fatalf("groups = %#v, want provider-side membership to recover the stable group", groups.GetGroups())
	}
}

func TestIAMDirectoryServiceGetGroup(t *testing.T) {
	svc := NewIAMDirectoryService(newDirectoryTestDeps())
	group, err := svc.GetGroup(newDirectoryContext(), &v1.GetGroupRequest{OrgId: "aisphere", GroupId: "dev-team"})
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	t.Logf("Group: %s", group.GetName())
}

type directoryAliasIdentity struct {
	authn.IdentityAdmin
}

func (directoryAliasIdentity) FindUsers(context.Context, authn.UserFilter) ([]authn.User, error) {
	return []authn.User{{
		ID:       "user-1",
		Username: "alice",
		Groups:   []string{"engineering"},
	}}, nil
}

func (directoryAliasIdentity) ListGroups(context.Context, authn.GroupFilter) ([]authn.Group, error) {
	return []authn.Group{{
		ID:          "grp_stable",
		ExternalID:  "engineering",
		OrgID:       "aisphere",
		Name:        "grp_stable",
		DisplayName: "Engineering",
	}}, nil
}

type directoryStaleAliasIdentity struct {
	authn.IdentityAdmin
}

func (directoryStaleAliasIdentity) FindUsers(context.Context, authn.UserFilter) ([]authn.User, error) {
	return []authn.User{{
		ID:       "user-1",
		Username: "alice",
		Groups:   []string{"legacy-engineering"},
	}}, nil
}

func (directoryStaleAliasIdentity) ListGroups(context.Context, authn.GroupFilter) ([]authn.Group, error) {
	return []authn.Group{{
		ID:          "grp_stable",
		OrgID:       "aisphere",
		Name:        "grp_stable",
		DisplayName: "Engineering",
		Users:       []string{"user-1"},
	}}, nil
}
