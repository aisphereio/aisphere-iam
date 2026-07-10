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
		relationships: store,
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
	group authn.Group
}

func (f fakeIdentityAdmin) CreateGroup(context.Context, authn.CreateGroupRequest) (authn.Group, error) {
	return f.group, nil
}

func (f fakeIdentityAdmin) UpdateGroup(context.Context, authn.UpdateGroupRequest) (authn.Group, error) {
	return f.group, nil
}

func (f fakeIdentityAdmin) DeleteGroup(context.Context, authn.DeleteGroupRequest) error {
	return nil
}

func (f fakeIdentityAdmin) AssignUserToGroup(context.Context, authn.AssignUserToGroupRequest) error {
	return nil
}

func (f fakeIdentityAdmin) RemoveUserFromGroup(context.Context, authn.AssignUserToGroupRequest) error {
	return nil
}
