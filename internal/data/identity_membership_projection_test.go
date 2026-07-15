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
	zoneMembership := zoneMemberRelationship("aisphere", "user-1")
	if _, err := store.WriteRelationships(ctx, membership, zoneMembership); err != nil {
		t.Fatalf("seed relationships: %v", err)
	}

	// fakeIdentityAdmin with no groupsForUser simulates the user having no
	// remaining groups in the zone after removal — zone#member should be
	// deleted too.
	admin := authzProjectingIdentityAdmin{
		IdentityAdmin: fakeIdentityAdmin{},
		projection:    NewIdentityProjectionDispatcher(store, store, nil, nil),
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
		t.Fatalf("qualified group membership was not removed: %#v", rels)
	}
	if containsRelationship(rels, zoneMembership) {
		t.Fatalf("zone membership should be removed when no groups remain: %#v", rels)
	}
}

// When a user belongs to multiple groups in a zone, removing them from one
// group must preserve zone#member so they retain zone-level visibility.
func TestAuthzProjectingIdentityAdminPreservesZoneMembershipWithMultipleGroups(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	groupAMembership := groupMemberRelationship("aisphere/engineering", "user-1")
	zoneMembership := zoneMemberRelationship("aisphere", "user-1")
	if _, err := store.WriteRelationships(ctx, groupAMembership, zoneMembership); err != nil {
		t.Fatalf("seed relationships: %v", err)
	}

	// Simulate the user still being a member of another group ("platform")
	// in the same zone after removal from "engineering".
	admin := authzProjectingIdentityAdmin{
		IdentityAdmin: fakeIdentityAdmin{
			groupsForUser: []authn.Group{{ID: "platform", OrgID: "aisphere"}},
		},
		projection: NewIdentityProjectionDispatcher(store, store, nil, nil),
	}
	if err := admin.RemoveUserFromGroup(ctx, authn.AssignUserToGroupRequest{
		OrgID:   "aisphere",
		GroupID: "engineering",
		UserID:  "user-1",
	}); err != nil {
		t.Fatalf("RemoveUserFromGroup returned error: %v", err)
	}

	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	if containsRelationship(rels, groupAMembership) {
		t.Fatalf("group membership was not removed: %#v", rels)
	}
	if !containsRelationship(rels, zoneMembership) {
		t.Fatalf("zone membership should be preserved when other groups remain: %#v", rels)
	}
}
