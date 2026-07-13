package resource

import (
	"context"
	"testing"

	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authz"
)

func TestResourceBindingLifecycleAndExternalLookup(t *testing.T) {
	ctx := context.Background()
	repo := data.NewMemoryControlPlaneRepository()
	store := authz.NewMemoryRelationshipStore()
	service := NewService(repo, store)

	for _, rt := range []*data.ResourceTypeModel{
		{Type: "skill", SpiceDBType: "skill", RelationsJSON: `["backing_repo"]`, Status: data.StatusActive},
		{Type: "git_repository", SpiceDBType: "git_repository", RelationsJSON: `[]`, Status: data.StatusActive},
	} {
		if err := repo.UpsertResourceType(ctx, rt); err != nil {
			t.Fatal(err)
		}
	}
	for _, resource := range []*data.ResourceModel{
		{Type: "skill", ID: "skill-1", OrgID: "zone-a", OwnerService: "hub", OwnerResourceID: "skill-1", Status: data.StatusActive},
		{Type: "git_repository", ID: "repo-1", OrgID: "zone-a", OwnerService: "git", OwnerResourceID: "repo-1", Status: data.StatusActive},
	} {
		if err := repo.UpsertResource(ctx, resource); err != nil {
			t.Fatal(err)
		}
	}

	binding, _, err := service.BindResource(ctx, BindResourceRequest{
		ID: "binding-1", Source: ResourceRef{Type: "skill", ID: "skill-1"},
		Relation: RelationBackingRepo, Target: ResourceRef{Type: "git_repository", ID: "repo-1"},
	})
	if err != nil {
		t.Fatalf("BindResource: %v", err)
	}
	rels, _ := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if len(rels) != 1 {
		t.Fatalf("relationships after bind = %#v", rels)
	}

	archived, wr, err := service.UnbindResource(ctx, UnbindResourceRequest{ID: binding.ID})
	if err != nil {
		t.Fatalf("UnbindResource: %v", err)
	}
	if archived.Status != data.StatusArchived || wr.Deleted != 1 {
		t.Fatalf("unbind result = (%#v, %#v)", archived, wr)
	}
	rels, _ = store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if len(rels) != 0 {
		t.Fatalf("relationships after unbind = %#v", rels)
	}
	if _, _, err := service.UnbindResource(ctx, UnbindResourceRequest{ID: binding.ID}); err != nil {
		t.Fatalf("idempotent unbind: %v", err)
	}

	if _, err := service.BindExternalResource(ctx, BindExternalResourceRequest{
		ID: "external-1", Resource: ResourceRef{Type: "skill", ID: "skill-1"},
		Provider: "gitlab", ExternalType: "project", ExternalID: "42",
	}); err != nil {
		t.Fatalf("BindExternalResource: %v", err)
	}
	page, err := repo.ListExternalResourceBindings(ctx, data.ListOptions{Provider: "gitlab", ExternalID: "42"})
	if err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("external lookup = (%#v, %v)", page, err)
	}
}
