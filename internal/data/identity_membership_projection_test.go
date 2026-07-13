package data

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

func TestAuthzProjectingIdentityAdminRemovesQualifiedGroupMembership(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	membership := groupMemberRelationship("aisphere/platform", "user-1")
	if _, err := store.WriteRelationships(ctx, membership); err != nil {
		t.Fatalf("seed relationship: %v", err)
	}

	admin := authzProjectingIdentityAdmin{
		IdentityAdmin: fakeIdentityAdmin{},
		projection:    NewIdentityProjectionDispatcher(store, nil, nil),
	}
	if err := admin.RemoveUserFromGroup(ctx, authn.AssignUserToGroupRequest{
		OrgID:   "aisphere",
		GroupID: "platform",
		UserID:  "user-1",
	}); err != nil {
		t.Fatalf("RemoveUserFromGroup returned error: %v", err)
	}

	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	if containsRelationship(rels, membership) {
		t.Fatalf("qualified membership was not removed: %#v", rels)
	}
}
