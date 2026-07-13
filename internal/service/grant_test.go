package service

import (
	"context"
	"testing"

	grantv1 "github.com/aisphereio/aisphere-iam/api/iam/grant/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	grantbiz "github.com/aisphereio/aisphere-iam/internal/biz/grant"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

// memoryAuthorizer wraps MemoryRelationshipStore to implement authz.Authorizer
type memoryAuthorizer struct {
	*authz.MemoryRelationshipStore
}

func (m memoryAuthorizer) Check(ctx context.Context, req authz.CheckRequest) (authz.Decision, error) {
	return authz.NewMemoryAuthorizer(m.MemoryRelationshipStore).Check(ctx, req)
}

func (m memoryAuthorizer) BatchCheck(ctx context.Context, req authz.BatchCheckRequest) (authz.BatchCheckResult, error) {
	out := authz.BatchCheckResult{Decisions: make([]authz.Decision, 0, len(req.Checks))}
	for _, check := range req.Checks {
		decision, err := m.Check(ctx, check)
		if err != nil {
			return authz.BatchCheckResult{}, err
		}
		out.Decisions = append(out.Decisions, decision)
	}
	return out, nil
}

func TestGrantServiceRegisterRoleTemplate(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	store := authz.NewMemoryRelationshipStore()
	authorizer := memoryAuthorizer{MemoryRelationshipStore: store}
	biz := grantbiz.NewService(repo, authorizer, store)
	svc := NewGrantService(biz, repo)
	ctx := context.Background()

	rt, err := svc.RegisterRoleTemplate(ctx, &grantv1.RegisterRoleTemplateRequest{
		RoleTemplate: &grantv1.RoleTemplate{
			ResourceType: "test_skill",
			RoleKey:      "owner",
			Relation:     "owner",
			DisplayName:  "Owner",
		},
	})
	if err != nil {
		t.Fatalf("RegisterRoleTemplate: %v", err)
	}
	if rt.GetRoleKey() != "owner" {
		t.Fatalf("unexpected role key: %s", rt.GetRoleKey())
	}
}

func TestGrantServiceListRoleTemplates(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	store := authz.NewMemoryRelationshipStore()
	authorizer := memoryAuthorizer{MemoryRelationshipStore: store}
	biz := grantbiz.NewService(repo, authorizer, store)
	svc := NewGrantService(biz, repo)
	ctx := context.Background()

	_, err := svc.RegisterRoleTemplate(ctx, &grantv1.RegisterRoleTemplateRequest{
		RoleTemplate: &grantv1.RoleTemplate{
			ResourceType: "test_skill",
			RoleKey:      "owner",
			Relation:     "owner",
			DisplayName:  "Owner",
		},
	})
	if err != nil {
		t.Fatalf("RegisterRoleTemplate: %v", err)
	}

	list, err := svc.ListRoleTemplates(ctx, &grantv1.ListRoleTemplatesRequest{
		ResourceType: "test_skill",
	})
	if err != nil {
		t.Fatalf("ListRoleTemplates: %v", err)
	}
	if len(list.GetRoleTemplates()) != 1 {
		t.Fatalf("expected 1 role template, got %d", len(list.GetRoleTemplates()))
	}
}

func TestGrantServiceGrantAndRevokeAccess(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	store := authz.NewMemoryRelationshipStore()
	authorizer := memoryAuthorizer{MemoryRelationshipStore: store}
	biz := grantbiz.NewService(repo, authorizer, store)
	svc := NewGrantService(biz, repo)
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID: "alice", SubjectType: "user", OrgID: "zone-a",
	})

	// Register resource type
	err := repo.UpsertResourceType(ctx, &data.ResourceTypeModel{
		Type: "test_skill", SpiceDBType: "test_skill",
		RelationsJSON:   `["owner"]`,
		PermissionsJSON: `["edit"]`,
	})
	if err != nil {
		t.Fatalf("UpsertResourceType: %v", err)
	}

	// Create the resource itself
	err = repo.UpsertResource(ctx, &data.ResourceModel{
		Type: "test_skill", ID: "skill-1", OrgID: "zone-a",
		OwnerService: "test", OwnerResourceID: "skill-1",
	})
	if err != nil {
		t.Fatalf("UpsertResource: %v", err)
	}

	// Register role template
	_, err = svc.RegisterRoleTemplate(ctx, &grantv1.RegisterRoleTemplateRequest{
		RoleTemplate: &grantv1.RoleTemplate{
			ResourceType: "test_skill",
			RoleKey:      "owner",
			Relation:     "owner",
			DisplayName:  "Owner",
		},
	})
	if err != nil {
		t.Fatalf("RegisterRoleTemplate: %v", err)
	}

	// Grant access
	grant, err := svc.GrantAccess(ctx, &grantv1.GrantAccessRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
		RoleKey:  "owner",
		Subject:  &resourcev1.SubjectRef{Type: "user", Id: "alice"},
	})
	if err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}
	if grant.GetId() == "" {
		t.Fatal("expected non-empty grant id")
	}

	// List grants
	grants, err := svc.ListGrants(ctx, &grantv1.ListGrantsRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
	})
	if err != nil {
		t.Fatalf("ListGrants: %v", err)
	}
	if len(grants.GetGrants()) != 1 {
		t.Fatalf("expected 1 grant, got %d", len(grants.GetGrants()))
	}

	// Revoke access
	revoked, err := svc.RevokeAccess(ctx, &grantv1.RevokeAccessRequest{
		GrantId: grant.GetId(),
	})
	if err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}
	if !revoked.GetRevoked() {
		t.Fatal("expected revoked=true")
	}
}

func TestGrantServiceExplainAccess(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	store := authz.NewMemoryRelationshipStore()
	authorizer := memoryAuthorizer{MemoryRelationshipStore: store}
	biz := grantbiz.NewService(repo, authorizer, store)
	svc := NewGrantService(biz, repo)
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID: "alice", SubjectType: "user", OrgID: "zone-a",
	})

	// Register resource type first
	err := repo.UpsertResourceType(ctx, &data.ResourceTypeModel{
		Type: "test_skill", SpiceDBType: "test_skill",
		RelationsJSON:   `["viewer"]`,
		PermissionsJSON: `["view"]`,
	})
	if err != nil {
		t.Fatalf("UpsertResourceType: %v", err)
	}

	explain, err := svc.ExplainAccess(ctx, &grantv1.ExplainAccessRequest{
		Subject:    &resourcev1.SubjectRef{Type: "user", Id: "alice"},
		Resource:   &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
		Permission: "view",
	})
	if err != nil {
		t.Fatalf("ExplainAccess: %v", err)
	}
	t.Logf("Explain: allowed=%v", explain.GetAllowed())
}