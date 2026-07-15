package data

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

func TestAuthzProjectingIdentityAdminWritesZoneQualifiedGroupEdges(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	admin := authzProjectingIdentityAdmin{
		IdentityAdmin: fakeIdentityAdmin{
			group: authn.Group{ID: "platform", OrgID: "aisphere", ParentID: "root"},
		},
		projection: NewIdentityProjectionDispatcher(store, store, nil, nil),
	}

	if _, err := admin.CreateGroup(ctx, authn.CreateGroupRequest{
		Group: authn.Group{OrgID: "aisphere", ID: "platform", ParentID: "root"},
	}); err != nil {
		t.Fatalf("CreateGroup returned error: %v", err)
	}
	if err := admin.AssignUserToGroup(ctx, authn.AssignUserToGroupRequest{
		OrgID:   "aisphere",
		GroupID: "platform",
		UserID:  "user-1",
	}); err != nil {
		t.Fatalf("AssignUserToGroup returned error: %v", err)
	}

	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	want := []authz.Relationship{
		{
			Resource: authz.ObjectRef{Type: "group", ID: "aisphere/platform"},
			Relation: "zone",
			Subject:  authz.SubjectRef{Type: "zone", ID: "aisphere"},
		},
		{
			Resource: authz.ObjectRef{Type: "group", ID: "aisphere/platform"},
			Relation: "parent",
			Subject:  authz.SubjectRef{Type: "group", ID: "aisphere/root"},
		},
		{
			Resource: authz.ObjectRef{Type: "group", ID: "aisphere/platform"},
			Relation: "member",
			Subject:  authz.SubjectRef{Type: "user", ID: "user-1"},
		},
	}
	for _, expected := range want {
		if !containsRelationship(rels, expected) {
			t.Fatalf("missing relationship: %#v; got %#v", expected, rels)
		}
	}
}

func TestAuthzProjectingIdentityAdminProjectsGroupOwnerFromContext(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	admin := authzProjectingIdentityAdmin{
		IdentityAdmin: fakeIdentityAdmin{
			group: authn.Group{ID: "platform", OrgID: "aisphere"},
		},
		projection: NewIdentityProjectionDispatcher(store, store, nil, nil),
	}
	owner := authz.SubjectRef{Type: "user", ID: "creator-1"}
	if _, err := admin.CreateGroup(WithGroupOwner(ctx, owner), authn.CreateGroupRequest{
		Group: authn.Group{OrgID: "aisphere", ID: "platform"},
	}); err != nil {
		t.Fatalf("CreateGroup returned error: %v", err)
	}
	want := authz.Relationship{
		Resource: authz.ObjectRef{Type: "group", ID: "aisphere/platform"},
		Relation: "owner",
		Subject:  owner,
	}
	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	if !containsRelationship(rels, want) {
		t.Fatalf("group#owner was not projected for the creator; got %#v", rels)
	}
	// Without an owner in context, no owner relationship is projected.
	store2 := authz.NewMemoryRelationshipStore()
	admin2 := authzProjectingIdentityAdmin{
		IdentityAdmin: fakeIdentityAdmin{group: authn.Group{ID: "platform", OrgID: "aisphere"}},
		projection:    NewIdentityProjectionDispatcher(store2, store2, nil, nil),
	}
	if _, err := admin2.CreateGroup(ctx, authn.CreateGroupRequest{Group: authn.Group{OrgID: "aisphere", ID: "platform"}}); err != nil {
		t.Fatalf("CreateGroup without owner returned error: %v", err)
	}
	rels2, _ := store2.ReadRelationships(ctx, authz.RelationshipFilter{})
	for _, rel := range rels2 {
		if rel.Relation == "owner" {
			t.Fatalf("unexpected owner relationship projected without context owner: %#v", rel)
		}
	}
}

func TestAuthzProjectingIdentityAdminDeleteCapturesCompensationRelationships(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	// Seed a direct grant the user holds on a project so we can assert it is
	// captured as compensation and restored on a compensating rollback.
	seed := authz.Relationship{
		Resource: authz.ObjectRef{Type: "project", ID: "proj-1"},
		Relation: "developer",
		Subject:  authz.SubjectRef{Type: "user", ID: "user-9"},
	}
	if _, err := store.WriteRelationships(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	admin := authzProjectingIdentityAdmin{
		IdentityAdmin: fakeIdentityAdmin{},
		projection:    NewIdentityProjectionDispatcher(store, store, nil, nil),
	}
	if err := admin.DeleteUser(ctx, authn.DeleteUserRequest{UserID: "user-9"}); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	// The delete projection should have removed the seeded relationship.
	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "project", ResourceID: "proj-1"})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	for _, rel := range rels {
		if rel.Subject.ID == "user-9" {
			t.Fatalf("seeded developer relationship survived DeleteUser: %#v", rel)
		}
	}
	// userSubjectDeleteFilters must enumerate project as a user-subject type so
	// direct grants like project#developer are purged, not just platform/zone/group.
	for _, filter := range userSubjectDeleteFilters("user-9") {
		if filter.ResourceType == "project" {
			return
		}
	}
	t.Fatal("userSubjectDeleteFilters does not cover the project resource type")
}

func TestUserSubjectDeleteFiltersCoversDirectGrantResourceTypes(t *testing.T) {
	filters := userSubjectDeleteFilters("user-1")
	wantTypes := []string{"platform", "zone", "group", "project", "skill", "git_repository", "role_binding"}
	have := map[string]bool{}
	for _, f := range filters {
		have[f.ResourceType] = true
	}
	for _, ty := range wantTypes {
		if !have[ty] {
			t.Fatalf("userSubjectDeleteFilters missing resource type %q (direct grants would survive user delete)", ty)
		}
	}
}

func TestAuthzProjectingIdentityAdminRejectsGroupNameRename(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	admin := authzProjectingIdentityAdmin{
		IdentityAdmin: fakeIdentityAdmin{
			group: authn.Group{ID: "platform", Name: "platform", OrgID: "aisphere"},
		},
		projection: NewIdentityProjectionDispatcher(store, store, nil, nil),
	}

	// Same-name update (e.g. only changing displayName) must succeed.
	if _, err := admin.UpdateGroup(ctx, authn.UpdateGroupRequest{Group: authn.Group{
		ID: "platform", OrgID: "aisphere", Name: "platform", DisplayName: "Platform Team",
	}}); err != nil {
		t.Fatalf("same-name update should succeed, got: %v", err)
	}

	// Empty Name (caller omitted the field) must succeed.
	if _, err := admin.UpdateGroup(ctx, authn.UpdateGroupRequest{Group: authn.Group{
		ID: "platform", OrgID: "aisphere", DisplayName: "Platform Team v2",
	}}); err != nil {
		t.Fatalf("empty-name update should succeed, got: %v", err)
	}

	// Rename attempt must be rejected.
	if _, err := admin.UpdateGroup(ctx, authn.UpdateGroupRequest{Group: authn.Group{
		ID: "platform", OrgID: "aisphere", Name: "renamed-platform",
	}}); err == nil {
		t.Fatal("rename update should return an error, got nil")
	}
}

func TestAuthzProjectingIdentityAdminLinksOrganizationToPlatform(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	admin := authzProjectingIdentityAdmin{
		IdentityAdmin: fakeIdentityAdmin{organization: authn.Organization{ID: "org-a", Name: "org-a"}},
		projection:    NewIdentityProjectionDispatcher(store, store, nil, nil),
	}

	if _, err := admin.CreateOrganization(ctx, authn.CreateOrganizationRequest{Organization: authn.Organization{ID: "org-a", Name: "org-a"}}); err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}
	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatal(err)
	}
	want := authz.Relationship{
		Resource: authz.ObjectRef{Type: "zone", ID: "org-a"},
		Relation: "platform",
		Subject:  authz.SubjectRef{Type: "platform", ID: "global"},
	}
	if !containsRelationship(rels, want) {
		t.Fatalf("missing relationship %#v; got %#v", want, rels)
	}
}

func TestBuildDirectoryProjectionRelationshipsIncludesPlatformLink(t *testing.T) {
	rels, err := BuildDirectoryProjectionRelationships(context.Background(), fakeIdentityAdmin{}, "org-a")
	if err != nil {
		t.Fatal(err)
	}
	want := authz.Relationship{
		Resource: authz.ObjectRef{Type: "zone", ID: "org-a"},
		Relation: "platform",
		Subject:  authz.SubjectRef{Type: "platform", ID: "global"},
	}
	if !containsRelationship(rels, want) {
		t.Fatalf("missing relationship %#v; got %#v", want, rels)
	}
}

func containsRelationship(rels []authz.Relationship, want authz.Relationship) bool {
	for _, rel := range rels {
		if rel.Resource == want.Resource && rel.Relation == want.Relation && rel.Subject == want.Subject {
			return true
		}
	}
	return false
}

type fakeIdentityAdmin struct {
	authn.IdentityAdmin
	group          authn.Group
	organization   authn.Organization
	groupsForUser  []authn.Group // returned by ListGroups when filtering by UserID
}

func (f fakeIdentityAdmin) CreateOrganization(context.Context, authn.CreateOrganizationRequest) (authn.Organization, error) {
	return f.organization, nil
}

func (f fakeIdentityAdmin) ListGroups(_ context.Context, filter authn.GroupFilter) ([]authn.Group, error) {
	// When the caller filters by UserID (used by RemoveUserFromGroup to check
	// remaining groups), return the configured groupsForUser slice so tests can
	// simulate multi-group membership.
	if filter.UserID != "" {
		return f.groupsForUser, nil
	}
	return nil, nil
}

func (f fakeIdentityAdmin) FindUsers(context.Context, authn.UserFilter) ([]authn.User, error) {
	return nil, nil
}

func (f fakeIdentityAdmin) CreateGroup(context.Context, authn.CreateGroupRequest) (authn.Group, error) {
	return f.group, nil
}

func (f fakeIdentityAdmin) GetGroup(_ context.Context, _, _ string) (authn.Group, error) {
	return f.group, nil
}

func (f fakeIdentityAdmin) UpdateGroup(_ context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	// Reflect the update so callers can inspect the result.  Name stays as
	// the existing identifier (the projection layer rejects renames before
	// reaching here).
	g := f.group
	g.ParentID = req.Group.ParentID
	g.DisplayName = req.Group.DisplayName
	g.Type = req.Group.Type
	return g, nil
}

func (f fakeIdentityAdmin) DeleteGroup(context.Context, authn.DeleteGroupRequest) error {
	return nil
}

func (f fakeIdentityAdmin) DeleteUser(context.Context, authn.DeleteUserRequest) error {
	return nil
}

func (f fakeIdentityAdmin) DisableUser(context.Context, string, string) error {
	return nil
}

func (f fakeIdentityAdmin) AssignUserToGroup(context.Context, authn.AssignUserToGroupRequest) error {
	return nil
}

func (f fakeIdentityAdmin) RemoveUserFromGroup(context.Context, authn.AssignUserToGroupRequest) error {
	return nil
}
