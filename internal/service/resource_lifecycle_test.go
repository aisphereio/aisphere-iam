package service

import (
	"context"
	"testing"

	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	resourcebiz "github.com/aisphereio/aisphere-iam/internal/biz/resource"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

func TestResourceServiceFullLifecycle(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	biz := resourcebiz.NewService(repo, authz.NewMemoryRelationshipStore())
	service := NewResourceService(biz, repo)
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID: "alice", SubjectType: "user", OrgID: "zone-a",
	})

	// Register resource type
	rt, err := service.RegisterResourceType(ctx, &resourcev1.RegisterResourceTypeRequest{
		ResourceType: &resourcev1.ResourceType{Type: "test_skill", Relations: []string{"owner", "viewer"}, Permissions: []string{"view", "edit"}},
	})
	if err != nil {
		t.Fatalf("RegisterResourceType: %v", err)
	}
	if rt.GetType() != "test_skill" {
		t.Fatalf("unexpected resource type: %#v", rt)
	}

	// Upsert resource
	created, err := service.UpsertResource(ctx, &resourcev1.UpsertResourceRequest{
		Resource: &resourcev1.Resource{Ref: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"}, OrgId: "zone-a", Slug: "my-skill", DisplayName: "My Skill"},
	})
	if err != nil {
		t.Fatalf("UpsertResource: %v", err)
	}
	if created.GetRef().GetId() != "skill-1" {
		t.Fatalf("unexpected resource id: %s", created.GetRef().GetId())
	}

	// Get resource
	got, err := service.GetResource(ctx, &resourcev1.GetResourceRequest{ResourceType: "test_skill", ResourceId: "skill-1"})
	if err != nil {
		t.Fatalf("GetResource: %v", err)
	}
	if got.GetDisplayName() != "My Skill" {
		t.Fatalf("unexpected display name: %s", got.GetDisplayName())
	}

	// Move resource
	moved, err := service.MoveResource(ctx, &resourcev1.MoveResourceRequest{
		ResourceType: "test_skill", ResourceId: "skill-1",
		NewParent: &resourcev1.ResourceRef{Type: "project", Id: "project-1"},
	})
	if err != nil {
		t.Fatalf("MoveResource: %v", err)
	}
	if moved.GetParent().GetId() != "project-1" {
		t.Fatalf("MoveResource did not update parent: %#v", moved.GetParent())
	}

	// Archive resource
	archived, err := service.ArchiveResource(ctx, &resourcev1.ArchiveResourceRequest{ResourceType: "test_skill", ResourceId: "skill-1"})
	if err != nil {
		t.Fatalf("ArchiveResource: %v", err)
	}
	if archived.GetStatus() != "archived" {
		t.Fatalf("expected archived status, got: %s", archived.GetStatus())
	}

	// Delete resource
	deleted, err := service.DeleteResource(ctx, &resourcev1.DeleteResourceRequest{ResourceType: "test_skill", ResourceId: "skill-1"})
	if err != nil {
		t.Fatalf("DeleteResource: %v", err)
	}
	if !deleted.GetDeleted() {
		t.Fatal("expected deleted=true")
	}
}

func TestResourceServiceBindUnbind(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	biz := resourcebiz.NewService(repo, authz.NewMemoryRelationshipStore())
	service := NewResourceService(biz, repo)
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID: "alice", SubjectType: "user", OrgID: "zone-a",
	})

	// Register resource type first
	_, err := service.RegisterResourceType(ctx, &resourcev1.RegisterResourceTypeRequest{
		ResourceType: &resourcev1.ResourceType{Type: "test_skill", Relations: []string{"owner"}, Permissions: []string{"view"}},
	})
	if err != nil {
		t.Fatalf("RegisterResourceType: %v", err)
	}

	// Create resource
	_, err = service.UpsertResource(ctx, &resourcev1.UpsertResourceRequest{
		Resource: &resourcev1.Resource{Ref: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"}, OrgId: "zone-a", DisplayName: "Skill 1"},
	})
	if err != nil {
		t.Fatalf("UpsertResource: %v", err)
	}

	// Create target resource for binding
	_, err = service.UpsertResource(ctx, &resourcev1.UpsertResourceRequest{
		Resource: &resourcev1.Resource{Ref: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-2"}, OrgId: "zone-a", DisplayName: "Target"},
	})
	if err != nil {
		t.Fatalf("UpsertResource target: %v", err)
	}

	// Bind resource
	binding, err := service.BindResource(ctx, &resourcev1.BindResourceRequest{
		Binding: &resourcev1.ResourceBinding{
			Source:   &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
			Relation: "owner",
			Target:   &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-2"},
		},
	})
	if err != nil {
		t.Fatalf("BindResource: %v", err)
	}
	if binding.GetId() == "" {
		t.Fatal("expected non-empty binding id")
	}

	// List bindings
	bindings, err := service.ListResourceBindings(ctx, &resourcev1.ListResourceBindingsRequest{
		Source: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
	})
	if err != nil {
		t.Fatalf("ListResourceBindings: %v", err)
	}
	if len(bindings.GetBindings()) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings.GetBindings()))
	}

	// Unbind resource
	unbound, err := service.UnbindResource(ctx, &resourcev1.UnbindResourceRequest{BindingId: binding.GetId()})
	if err != nil {
		t.Fatalf("UnbindResource: %v", err)
	}
	if !unbound.GetUnbound() {
		t.Fatal("expected unbound=true")
	}

	// Verify binding is gone
	bindings, err = service.ListResourceBindings(ctx, &resourcev1.ListResourceBindingsRequest{
		Source: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
	})
	if err != nil {
		t.Fatalf("ListResourceBindings after unbind: %v", err)
	}
	if len(bindings.GetBindings()) != 0 {
		t.Fatalf("expected 0 bindings after unbind, got %d", len(bindings.GetBindings()))
	}
}

func TestResourceServiceExternalBindings(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	biz := resourcebiz.NewService(repo, authz.NewMemoryRelationshipStore())
	service := NewResourceService(biz, repo)
	ctx := context.Background()

	// Register resource type
	_, err := service.RegisterResourceType(ctx, &resourcev1.RegisterResourceTypeRequest{
		ResourceType: &resourcev1.ResourceType{Type: "test_skill", Relations: []string{"owner"}, Permissions: []string{"view"}},
	})
	if err != nil {
		t.Fatalf("RegisterResourceType: %v", err)
	}

	// Create resource
	ctx2 := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID: "alice", SubjectType: "user", OrgID: "zone-a",
	})
	_, err = service.UpsertResource(ctx2, &resourcev1.UpsertResourceRequest{
		Resource: &resourcev1.Resource{Ref: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"}, OrgId: "zone-a"},
	})
	if err != nil {
		t.Fatalf("UpsertResource: %v", err)
	}

// Bind external resource
		_, err = service.BindExternalResource(ctx2, &resourcev1.BindExternalResourceRequest{
			Binding: &resourcev1.ExternalResourceBinding{
				Resource:     &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
				Provider:     "github",
				ExternalType: "repository",
				ExternalId:   "aisphereio/demo",
				ExternalUrl:  "https://github.com/aisphereio/demo",
			},
		})
		if err != nil {
			t.Fatalf("BindExternalResource: %v", err)
		}

		// List external bindings
		bindings, err := service.ListExternalResourceBindings(ctx2, &resourcev1.ListExternalResourceBindingsRequest{
			Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
		})
	if err != nil {
		t.Fatalf("ListExternalResourceBindings: %v", err)
	}
	if len(bindings.GetBindings()) != 1 {
		t.Fatalf("expected 1 external binding, got %d", len(bindings.GetBindings()))
	}
	if bindings.GetBindings()[0].GetExternalId() != "aisphereio/demo" {
		t.Fatalf("unexpected external id: %s", bindings.GetBindings()[0].GetExternalId())
	}
}