package service

import (
	"context"
	"testing"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

func newAuthzAdminDeps() IAMDeps {
	store := authz.NewMemoryRelationshipStore()
	store.WriteRelationships(context.Background(),
		authz.Relationship{Resource: authz.ObjectRef{Type: "iam_authz", ID: "global"}, Relation: "view_relationships", Subject: authz.SubjectRef{Type: "user", ID: "admin"}},
		authz.Relationship{Resource: authz.ObjectRef{Type: "iam_authz", ID: "global"}, Relation: "view_schema", Subject: authz.SubjectRef{Type: "user", ID: "admin"}},
		authz.Relationship{Resource: authz.ObjectRef{Type: "iam_authz", ID: "global"}, Relation: "publish_schema", Subject: authz.SubjectRef{Type: "user", ID: "admin"}},
	)
	return IAMDeps{Authz: memoryAuthzAdmin{MemoryRelationshipStore: store}}
}

func newAdminContext() context.Context {
	return authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID: "admin", SubjectType: "user", OrgID: "aisphere",
	})
}

func TestAuthorizationAdminGetSchema(t *testing.T) {
	svc := NewIAMAuthorizationAdminService(newAuthzAdminDeps())
	schema, err := svc.GetAuthorizationSchema(newAdminContext(), &v1.GetAuthorizationSchemaRequest{})
	if err != nil {
		t.Fatalf("GetAuthorizationSchema: %v", err)
	}
	t.Logf("Schema version: %s, text length: %d", schema.GetVersion(), len(schema.GetText()))
}

func TestAuthorizationAdminValidateSchema(t *testing.T) {
	svc := NewIAMAuthorizationAdminService(newAuthzAdminDeps())
	result, err := svc.ValidateAuthorizationSchema(newAdminContext(), &v1.ValidateAuthorizationSchemaRequest{
		Text: "definition user {}\ndefinition group {\nrelation member: user\n}",
	})
	if err != nil {
		t.Fatalf("ValidateAuthorizationSchema: %v", err)
	}
	if !result.GetValid() {
		t.Fatalf("expected valid schema, got error: %s", result.GetError())
	}
}

func TestAuthorizationAdminCheckPermission(t *testing.T) {
	svc := NewIAMAuthorizationAdminService(newAuthzAdminDeps())
	result, err := svc.CheckAuthorization(newAdminContext(), &v1.CheckPermissionRequest{
		Subject:    &v1.SubjectRef{Type: "user", Id: "alice"},
		Resource:   &v1.ObjectRef{Type: "zone", Id: "aisphere"},
		Permission: "view_zone",
	})
	if err != nil {
		t.Fatalf("CheckAuthorization: %v", err)
	}
	t.Logf("Check result: allowed=%v, effect=%s", result.GetAllowed(), result.GetEffect())
}

func TestAuthorizationAdminExplainAuthorization(t *testing.T) {
	svc := NewIAMAuthorizationAdminService(newAuthzAdminDeps())
	result, err := svc.ExplainAuthorization(newAdminContext(), &v1.CheckPermissionRequest{
		Subject:    &v1.SubjectRef{Type: "user", Id: "alice"},
		Resource:   &v1.ObjectRef{Type: "zone", Id: "aisphere"},
		Permission: "view_zone",
	})
	if err != nil {
		t.Fatalf("ExplainAuthorization: %v", err)
	}
	t.Logf("Explain: allowed=%v, steps=%v", result.GetAllowed(), result.GetSteps())
}

func TestAuthorizationAdminGetEffectivePermissions(t *testing.T) {
	svc := NewIAMAuthorizationAdminService(newAuthzAdminDeps())
	result, err := svc.GetEffectivePermissions(newAdminContext(), &v1.GetEffectivePermissionsRequest{
		SubjectType: "user", SubjectId: "alice",
		ResourceType: "zone", ResourceId: "aisphere",
	})
	if err != nil {
		t.Fatalf("GetEffectivePermissions: %v", err)
	}
	t.Logf("Effective permissions: %v", result.GetPermissions())
}