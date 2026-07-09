package data

import (
	"context"
	"testing"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

func TestIdentityGroupTopologyRelationships(t *testing.T) {
	rels := groupTopologyRelationships(authn.Group{ID: "dev", OrgID: "aisphere", ParentID: "platform"}, authn.Group{})
	assertHasRelationship(t, rels, authz.Relationship{
		Resource: authz.ObjectRef{Type: "group", ID: "dev"},
		Relation: "zone",
		Subject:  authz.SubjectRef{Type: "zone", ID: "aisphere"},
	})
	assertHasRelationship(t, rels, authz.Relationship{
		Resource: authz.ObjectRef{Type: "group", ID: "dev"},
		Relation: "parent",
		Subject:  authz.SubjectRef{Type: "group", ID: "platform"},
	})
	assertHasRelationship(t, rels, authz.Relationship{
		Resource: authz.ObjectRef{Type: "group", ID: "platform"},
		Relation: "member",
		Subject:  authz.SubjectRef{Type: "group", ID: "dev", Relation: "member"},
	})
}

func TestIdentityAuthZProjectionApplyAndCompensate(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	payload := IdentityAuthZProjectionPayload{
		Operation: identityAuthZProjectionOperationUp,
		Relationships: []authz.Relationship{{
			Resource: authz.ObjectRef{Type: "group", ID: "dev"},
			Relation: "member",
			Subject:  authz.SubjectRef{Type: "user", ID: "u1"},
		}},
	}
	if _, err := ApplyIdentityAuthZProjection(ctx, store, payload); err != nil {
		t.Fatalf("ApplyIdentityAuthZProjection returned error: %v", err)
	}
	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "group", ResourceID: "dev", Relation: "member", SubjectType: "user", SubjectID: "u1"})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relationship, got %d", len(rels))
	}
	if _, err := CompensateIdentityAuthZProjection(ctx, store, payload); err != nil {
		t.Fatalf("CompensateIdentityAuthZProjection returned error: %v", err)
	}
	rels, err = store.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "group", ResourceID: "dev", Relation: "member", SubjectType: "user", SubjectID: "u1"})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	if len(rels) != 0 {
		t.Fatalf("expected relationship to be removed, got %d", len(rels))
	}
}

func assertHasRelationship(t *testing.T, rels []authz.Relationship, want authz.Relationship) {
	t.Helper()
	for _, got := range rels {
		if got.Resource == want.Resource && got.Relation == want.Relation && got.Subject == want.Subject {
			return
		}
	}
	t.Fatalf("relationship not found: %#v in %#v", want, rels)
}
