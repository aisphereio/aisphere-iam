package service

import (
	"context"
	"strings"
	"testing"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

func TestIAMAuthServiceGetMeReturnsPrincipalFromContext(t *testing.T) {
	svc := NewIAMAuthService(IAMDeps{})

	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID:   "user-1",
		SubjectType: authn.SubjectTypeUser,
		Provider:    "casdoor",
		OrgID:       "aisphere",
		Username:    "alice",
	})
	reply, err := svc.GetMe(ctx, &v1.GetMeRequest{})

	if err != nil {
		t.Fatalf("GetMe returned error: %v", err)
	}
	if reply.Principal.SubjectId != "user-1" {
		t.Fatalf("subject id = %q", reply.Principal.SubjectId)
	}
	if reply.Principal.Username != "alice" {
		t.Fatalf("username = %q", reply.Principal.Username)
	}
}

func TestIAMAuthServiceGetMeRequiresGatewayPrincipal(t *testing.T) {
	svc := NewIAMAuthService(IAMDeps{})

	_, err := svc.GetMe(context.Background(), &v1.GetMeRequest{IncludeProfile: true})

	if err == nil {
		t.Fatal("GetMe expected gateway principal error")
	}
	if !strings.Contains(err.Error(), "gateway principal is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIAMPermissionServiceWritesAndChecksRelationship(t *testing.T) {
	store := authz.NewMemoryRelationshipStore()
	admin := memoryAuthzAdmin{MemoryRelationshipStore: store}
	svc := NewIAMPermissionService(IAMDeps{Authz: admin})

	_, err := svc.WriteRelationship(context.Background(), &v1.WriteRelationshipRequest{
		Relationship: &v1.Relationship{
			Resource: &v1.ObjectRef{Type: "organization", Id: "aisphere"},
			Relation: "read",
			Subject:  &v1.SubjectRef{Type: "user", Id: "user-1"},
		},
	})
	if err != nil {
		t.Fatalf("WriteRelationship returned error: %v", err)
	}

	reply, err := svc.CheckPermission(context.Background(), &v1.CheckPermissionRequest{
		Subject:    &v1.SubjectRef{Type: "user", Id: "user-1"},
		Resource:   &v1.ObjectRef{Type: "organization", Id: "aisphere"},
		Permission: "read",
	})
	if err != nil {
		t.Fatalf("CheckPermission returned error: %v", err)
	}
	if !reply.Allowed {
		t.Fatalf("permission denied: %+v", reply)
	}
}

type memoryAuthzAdmin struct {
	*authz.MemoryRelationshipStore
}

func (m memoryAuthzAdmin) Check(ctx context.Context, req authz.CheckRequest) (authz.Decision, error) {
	return authz.NewMemoryAuthorizer(m.MemoryRelationshipStore).Check(ctx, req)
}
func (m memoryAuthzAdmin) BatchCheck(ctx context.Context, req authz.BatchCheckRequest) (authz.BatchCheckResult, error) {
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
func (m memoryAuthzAdmin) LookupResources(context.Context, authz.LookupResourcesRequest) (authz.LookupResourcesResult, error) {
	return authz.LookupResourcesResult{}, nil
}
func (m memoryAuthzAdmin) LookupSubjects(context.Context, authz.LookupSubjectsRequest) (authz.LookupSubjectsResult, error) {
	return authz.LookupSubjectsResult{}, nil
}
func (m memoryAuthzAdmin) ReadSchema(context.Context) (authz.Schema, error) {
	return authz.Schema{}, nil
}
func (m memoryAuthzAdmin) WriteSchema(context.Context, authz.Schema) error { return nil }
func (m memoryAuthzAdmin) ValidateSchema(context.Context, authz.Schema) error {
	return nil
}
