package service

import (
	"context"
	"strings"
	"testing"
	"time"

	grantv1 "github.com/aisphereio/aisphere-iam/api/iam/grant/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	grantbiz "github.com/aisphereio/aisphere-iam/internal/biz/grant"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func TestGrantServiceRegisterCustomRoleUsesPermissionsInsteadOfRelation(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	store := authz.NewMemoryRelationshipStore()
	biz := grantbiz.NewService(repo, memoryAuthorizer{MemoryRelationshipStore: store}, store)
	svc := NewGrantService(biz, repo)
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{SubjectID: "admin", SubjectType: "user"})
	if err := repo.UpsertResourceType(ctx, &data.ResourceTypeModel{
		Type: "test_skill", SpiceDBType: "test_skill", Grantable: true,
		RelationsJSON: `["owner","custom_binding"]`, PermissionsJSON: `["view","review"]`,
	}); err != nil {
		t.Fatal(err)
	}

	role, err := svc.RegisterRoleTemplate(ctx, &grantv1.RegisterRoleTemplateRequest{RoleTemplate: &grantv1.RoleTemplate{
		ResourceType: "test_skill", RoleKey: "reviewer", DisplayName: "Reviewer",
		Permissions: []string{"review", "view"},
	}})
	if err != nil {
		t.Fatalf("RegisterRoleTemplate: %v", err)
	}
	if role.GetRelation() != "custom_binding" || role.GetVersion() != 1 {
		t.Fatalf("role = %#v", role)
	}
	if got := role.GetPermissions(); len(got) != 2 || got[0] != "review" || got[1] != "view" {
		t.Fatalf("permissions = %v", got)
	}
}

func TestGrantServiceUpdatesCustomRoleWithImpactPreviewAndVersionCheck(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	store := authz.NewMemoryRelationshipStore()
	biz := grantbiz.NewService(repo, memoryAuthorizer{MemoryRelationshipStore: store}, store)
	svc := NewGrantService(biz, repo)
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{SubjectID: "admin", SubjectType: "user"})
	if err := repo.UpsertResourceType(ctx, &data.ResourceTypeModel{
		Type: "test_skill", SpiceDBType: "test_skill", Grantable: true,
		RelationsJSON: `["custom_binding"]`, PermissionsJSON: `["view","review","edit"]`,
	}); err != nil {
		t.Fatal(err)
	}
	created, err := svc.RegisterRoleTemplate(ctx, &grantv1.RegisterRoleTemplateRequest{RoleTemplate: &grantv1.RoleTemplate{
		ResourceType: "test_skill", RoleKey: "reviewer", DisplayName: "Reviewer", Permissions: []string{"view", "review"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	impact, err := svc.PreviewRoleTemplateImpact(ctx, &grantv1.PreviewRoleTemplateImpactRequest{Id: created.GetId(), Permissions: []string{"view", "edit"}})
	if err != nil {
		t.Fatal(err)
	}
	if impact.GetActiveGrantCount() != 0 || len(impact.GetAddedPermissions()) != 1 || impact.GetAddedPermissions()[0] != "edit" || len(impact.GetRemovedPermissions()) != 1 || impact.GetRemovedPermissions()[0] != "review" {
		t.Fatalf("impact = %#v", impact)
	}

	updated, err := svc.UpdateRoleTemplate(ctx, &grantv1.UpdateRoleTemplateRequest{
		Id: created.GetId(), DisplayName: "Skill reviewer", Permissions: []string{"view", "edit"}, ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetVersion() != 2 || updated.GetDisplayName() != "Skill reviewer" {
		t.Fatalf("updated = %#v", updated)
	}
	if _, err := svc.UpdateRoleTemplate(ctx, &grantv1.UpdateRoleTemplateRequest{Id: created.GetId(), Permissions: []string{"view"}, ExpectedVersion: 1}); err == nil || !strings.Contains(err.Error(), "version conflict") {
		t.Fatalf("error = %v, want version conflict", err)
	}
}

func TestGrantServiceCustomRoleGrantUsesRoleBindingAndRevokesAllBindingEdges(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	store := authz.NewMemoryRelationshipStore()
	biz := grantbiz.NewService(repo, memoryAuthorizer{MemoryRelationshipStore: store}, store)
	svc := NewGrantService(biz, repo)
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{SubjectID: "admin", SubjectType: "user"})
	if err := repo.UpsertResourceType(ctx, &data.ResourceTypeModel{
		Type: "test_skill", SpiceDBType: "test_skill", Grantable: true,
		RelationsJSON: `["custom_binding"]`, PermissionsJSON: `["view","review"]`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertResource(ctx, &data.ResourceModel{Type: "test_skill", ID: "skill-1", OrgID: "org-a", OwnerService: "test", OwnerResourceID: "skill-1"}); err != nil {
		t.Fatal(err)
	}
	role, err := svc.RegisterRoleTemplate(ctx, &grantv1.RegisterRoleTemplateRequest{RoleTemplate: &grantv1.RoleTemplate{
		ResourceType: "test_skill", RoleKey: "reviewer", DisplayName: "Reviewer", Permissions: []string{"view", "review"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := svc.GrantAccess(ctx, &grantv1.GrantAccessRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"}, RoleKey: "reviewer",
		Subject: &resourcev1.SubjectRef{Type: "user", Id: "alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []authz.Relationship{
		{Resource: authz.ObjectRef{Type: "role_binding", ID: grant.GetId()}, Relation: "role", Subject: authz.SubjectRef{Type: "custom_role", ID: "test_skill:reviewer"}},
		{Resource: authz.ObjectRef{Type: "role_binding", ID: grant.GetId()}, Relation: "grantee", Subject: authz.SubjectRef{Type: "user", ID: "alice"}},
		{Resource: authz.ObjectRef{Type: "test_skill", ID: "skill-1"}, Relation: "custom_binding", Subject: authz.SubjectRef{Type: "role_binding", ID: grant.GetId()}},
	}
	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, relationship := range want {
		if !grantTestContainsRelationship(rels, relationship) {
			t.Fatalf("missing relationship %#v in %#v", relationship, rels)
		}
	}
	list, err := svc.ListRoleTemplates(ctx, &grantv1.ListRoleTemplatesRequest{ResourceType: "test_skill"})
	if err != nil || len(list.GetRoleTemplates()) != 1 || list.GetRoleTemplates()[0].GetActiveGrantCount() != 1 {
		t.Fatalf("roles = %#v, err = %v", list, err)
	}
	if _, err := svc.DisableRoleTemplate(ctx, &grantv1.DisableRoleTemplateRequest{Id: role.GetId(), ExpectedVersion: 1}); err == nil || !strings.Contains(err.Error(), "active grants") {
		t.Fatalf("error = %v, want active grant confirmation", err)
	}
	if _, err := svc.RevokeAccess(ctx, &grantv1.RevokeAccessRequest{GrantId: grant.GetId()}); err != nil {
		t.Fatal(err)
	}
	rels, err = store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, relationship := range want {
		if grantTestContainsRelationship(rels, relationship) {
			t.Fatalf("binding relationship remains after revoke: %#v", relationship)
		}
	}
}

func grantTestContainsRelationship(rels []authz.Relationship, want authz.Relationship) bool {
	for _, rel := range rels {
		if rel.Resource == want.Resource && rel.Relation == want.Relation && rel.Subject == want.Subject {
			return true
		}
	}
	return false
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

func TestGrantServiceExpireDueGrants(t *testing.T) {
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

	// Create the resource
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

	// Grant access with an expires_at in the past (already expired)
	past := timestamppb.New(time.Now().UTC().Add(-1 * time.Hour))
	grant, err := svc.GrantAccess(ctx, &grantv1.GrantAccessRequest{
		Resource:  &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
		RoleKey:   "owner",
		Subject:   &resourcev1.SubjectRef{Type: "user", Id: "alice"},
		ExpiresAt: past,
	})
	if err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}
	if grant.GetId() == "" {
		t.Fatal("expected non-empty grant id")
	}

	// Grant access without expiry (should not be affected)
	_, err = svc.GrantAccess(ctx, &grantv1.GrantAccessRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
		RoleKey:  "owner",
		Subject:  &resourcev1.SubjectRef{Type: "user", Id: "bob"},
	})
	if err != nil {
		t.Fatalf("GrantAccess (no expiry): %v", err)
	}

	// Grant access with future expiry (should not be affected)
	future := timestamppb.New(time.Now().UTC().Add(24 * time.Hour))
	_, err = svc.GrantAccess(ctx, &grantv1.GrantAccessRequest{
		Resource:  &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
		RoleKey:   "owner",
		Subject:   &resourcev1.SubjectRef{Type: "user", Id: "charlie"},
		ExpiresAt: future,
	})
	if err != nil {
		t.Fatalf("GrantAccess (future expiry): %v", err)
	}

	// Verify all 3 grants exist before expiry
	grants, err := svc.ListGrants(ctx, &grantv1.ListGrantsRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
	})
	if err != nil {
		t.Fatalf("ListGrants: %v", err)
	}
	if len(grants.GetGrants()) != 3 {
		t.Fatalf("expected 3 grants before expiry, got %d", len(grants.GetGrants()))
	}

	// Run expiry
	err = biz.ExpireDueGrants(ctx)
	if err != nil {
		t.Fatalf("ExpireDueGrants: %v", err)
	}

	// List grants again — ListGrants filters out revoked grants, so only 2 should remain
	grantsAfter, err := svc.ListGrants(ctx, &grantv1.ListGrantsRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
	})
	if err != nil {
		t.Fatalf("ListGrants after expiry: %v", err)
	}
	if len(grantsAfter.GetGrants()) != 2 {
		t.Fatalf("expected 2 active grants after expiry (expired one revoked), got %d", len(grantsAfter.GetGrants()))
	}

	// The remaining grants should be the no-expiry and future-expiry ones
	for _, g := range grantsAfter.GetGrants() {
		if g.GetId() == grant.GetId() {
			t.Fatal("expired grant should not appear in ListGrants (revoked)")
		}
	}

	// Run ExpireDueGrants again — should be idempotent
	err = biz.ExpireDueGrants(ctx)
	if err != nil {
		t.Fatalf("ExpireDueGrants (idempotent): %v", err)
	}

	// Still 2 grants after idempotent re-run
	grantsAfterIdempotent, err := svc.ListGrants(ctx, &grantv1.ListGrantsRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
	})
	if err != nil {
		t.Fatalf("ListGrants after idempotent expiry: %v", err)
	}
	if len(grantsAfterIdempotent.GetGrants()) != 2 {
		t.Fatalf("expected 2 grants after idempotent re-run, got %d", len(grantsAfterIdempotent.GetGrants()))
	}
}

func TestGrantServiceExpireDueGrants_NoExpiryGrants(t *testing.T) {
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

	// Create the resource
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

	// Create a grant with no expiry
	_, err = svc.GrantAccess(ctx, &grantv1.GrantAccessRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
		RoleKey:  "owner",
		Subject:  &resourcev1.SubjectRef{Type: "user", Id: "alice"},
	})
	if err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	// Run expiry — should not affect grants without expires_at
	err = biz.ExpireDueGrants(ctx)
	if err != nil {
		t.Fatalf("ExpireDueGrants: %v", err)
	}

	// Verify the grant is still active
	grants, err := svc.ListGrants(ctx, &grantv1.ListGrantsRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_skill", Id: "skill-1"},
	})
	if err != nil {
		t.Fatalf("ListGrants: %v", err)
	}
	if len(grants.GetGrants()) != 1 {
		t.Fatalf("expected 1 grant, got %d", len(grants.GetGrants()))
	}
	if grants.GetGrants()[0].GetRevokedAt() != nil {
		t.Fatal("grant without expiry should not be revoked")
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
