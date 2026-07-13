// Integration tests against aisphere-iam deployed on aisphere-dev (36.137.200.194).
//
// These tests call the deployed IAM service via gRPC with trusted headers
// to simulate an authenticated admin, verifying real business flows through
// real Casdoor, SpiceDB, and PostgreSQL.
//
// Run with:
//
//	go test -tags=integration ./internal/service/ -run TestIntegration -v -count=1
//
// Requires: configs/config.test.yaml with valid remote service credentials.
// Network access to 36.137.200.194:19080 (gRPC) and 36.137.200.194:30080 (PostgreSQL).

//go:build integration

package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	grantv1 "github.com/aisphereio/aisphere-iam/api/iam/grant/v1"
	projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	iamv1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/configx"
	"github.com/aisphereio/kernel/configx/file"
	"github.com/aisphereio/kernel/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ── Helpers ──

func getTestConfigPath() string {
	if p := os.Getenv("IAM_TEST_CONFIG"); p != "" {
		return p
	}
	return "../../configs/config.test.yaml"
}

// adminContext creates a context with trusted headers for the admin user.
func adminContext() context.Context {
	p := authn.Principal{
		SubjectID:   "admin",
		SubjectType: authn.SubjectTypeUser,
		OrgID:       "aisphere",
		Provider:    "casdoor",
	}
	headers := map[string]string{}
	authn.InjectTrustedHeaders(headers, p)
	md := metadata.New(headers)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// loadTestConfig loads the test configuration using configx.
func loadTestConfig(t *testing.T) conf.Bootstrap {
	t.Helper()
	path := getTestConfigPath()
	cfg := configx.New(configx.WithSource(file.NewSource(path)))
	if err := cfg.Load(); err != nil {
		t.Fatalf("config load: %v", err)
	}
	var bc conf.Bootstrap
	if err := cfg.Scan(&bc); err != nil {
		t.Fatalf("config scan: %v", err)
	}
	return bc
}

// newGRPCConn dials the deployed IAM gRPC endpoint.
func newGRPCConn(t *testing.T) *grpc.ClientConn {
	t.Helper()
	bc := loadTestConfig(t)
	addr := bc.Server.GRPC.Addr
	if addr == "" {
		addr = "36.137.200.194:19080"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("grpc.Dial(%s): %v", addr, err)
	}
	return conn
}

// newResources creates a Resources instance connected to the test database.
func newResources(t *testing.T) (*data.Resources, func()) {
	t.Helper()
	bc := loadTestConfig(t)
	logger := logx.DefaultLogger()
	resources, cleanup, err := data.NewResources(context.Background(), bc, data.ResourceOptions{Logger: logger})
	if err != nil {
		t.Fatalf("NewResources: %v", err)
	}
	return resources, cleanup
}

func tsSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
}

// ── 1. Service Health ──

func TestIntegrationConfigLoads(t *testing.T) {
	bc := loadTestConfig(t)
	if bc.Security.Authn.Provider != "casdoor" {
		t.Fatalf("expected casdoor provider, got: %s", bc.Security.Authn.Provider)
	}
	if bc.Data.Database.Config.DSN == "" {
		t.Fatal("expected non-empty DSN")
	}
	t.Logf("DSN: %s", bc.Data.Database.Config.DSN)
}

func TestIntegrationPostgresConnection(t *testing.T) {
	resources, cleanup := newResources(t)
	defer cleanup()
	if resources.DB == nil {
		t.Fatal("expected non-nil DB")
	}
	if err := resources.DB.PingContext(context.Background()); err != nil {
		t.Fatalf("DB Ping: %v", err)
	}
	t.Log("PostgreSQL connection OK")
}

func TestIntegrationCasdoorConnection(t *testing.T) {
	resources, cleanup := newResources(t)
	defer cleanup()
	if resources.Authn == nil {
		t.Fatal("expected non-nil Authn")
	}
	user, err := resources.Identity.GetUser(context.Background(), "aisphere", "admin")
	if err != nil {
		t.Fatalf("GetUser(admin): %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("expected username 'admin', got: %s", user.Username)
	}
	t.Logf("Casdoor connection OK, user: %s", user.Username)
}

func TestIntegrationSpiceDBConnection(t *testing.T) {
	resources, cleanup := newResources(t)
	defer cleanup()
	if resources.Authz == nil {
		t.Fatal("expected authorizer")
	}
	decision, err := resources.Authz.Check(context.Background(), authz.CheckRequest{
		Subject:    authz.SubjectRef{Type: "user", ID: "admin"},
		Resource:   authz.ObjectRef{Type: "zone", ID: "aisphere"},
		Permission: "view_zone",
	})
	if err != nil {
		t.Fatalf("SpiceDB Check: %v", err)
	}
	t.Logf("SpiceDB check: allowed=%v, effect=%s", decision.IsAllowed(), decision.Effect)
}

// ── 2. AUTHN: Authentication ──

func TestIntegrationAuthnGetMe(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMAuthServiceClient(conn)

	reply, err := client.GetMe(adminContext(), &iamv1.GetMeRequest{})
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if reply.GetPrincipal().GetSubjectId() != "admin" {
		t.Fatalf("expected subject 'admin', got: %s", reply.GetPrincipal().GetSubjectId())
	}
	t.Logf("GetMe OK: subject=%s, org=%s", reply.GetPrincipal().GetSubjectId(), reply.GetPrincipal().GetOrgId())
}

func TestIntegrationAuthnGetMeUnauthenticated(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMAuthServiceClient(conn)

	_, err := client.GetMe(context.Background(), &iamv1.GetMeRequest{})
	if err == nil {
		t.Fatal("expected error for unauthenticated GetMe")
	}
	t.Logf("GetMe correctly rejected unauthenticated: %v", err)
}

// ── 3. DIR: Directory Reads ──

func TestIntegrationDirectoryGetUser(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMDirectoryServiceClient(conn)

	user, err := client.GetUser(adminContext(), &iamv1.GetUserRequest{
		OrgId:  "aisphere",
		UserId: "admin",
	})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.GetUsername() != "admin" {
		t.Fatalf("expected username 'admin', got: %s", user.GetUsername())
	}
	t.Logf("GetUser OK: id=%s, username=%s", user.GetId(), user.GetUsername())
}

func TestIntegrationDirectoryListUsers(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMDirectoryServiceClient(conn)

	reply, err := client.ListUsers(adminContext(), &iamv1.ListUsersRequest{
		OrgId: "aisphere",
	})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(reply.GetUsers()) == 0 {
		t.Fatal("expected at least 1 user")
	}
	t.Logf("ListUsers OK: %d users", len(reply.GetUsers()))
}

func TestIntegrationDirectoryGetOrganization(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMDirectoryServiceClient(conn)

	org, err := client.GetOrganization(adminContext(), &iamv1.GetOrganizationRequest{
		OrgId: "aisphere",
	})
	if err != nil {
		t.Fatalf("GetOrganization: %v", err)
	}
	if org.GetName() != "aisphere" {
		t.Fatalf("expected org 'aisphere', got: %s", org.GetName())
	}
	t.Logf("GetOrganization OK: name=%s, display=%s", org.GetName(), org.GetDisplayName())
}

func TestIntegrationDirectoryListGroups(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMDirectoryServiceClient(conn)

	reply, err := client.ListGroups(adminContext(), &iamv1.ListGroupsRequest{
		OrgId: "aisphere",
	})
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	t.Logf("ListGroups OK: %d groups", len(reply.GetGroups()))
}

// ── 4. AUTHZ-RT: Runtime Authorization ──

func TestIntegrationPermissionCheck(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMPermissionServiceClient(conn)

	reply, err := client.CheckPermission(adminContext(), &iamv1.CheckPermissionRequest{
		Subject:    &iamv1.SubjectRef{Type: "user", Id: "admin"},
		Resource:   &iamv1.ObjectRef{Type: "zone", Id: "aisphere"},
		Permission: "view_zone",
	})
	if err != nil {
		t.Fatalf("CheckPermission: %v", err)
	}
	t.Logf("CheckPermission OK: allowed=%v, effect=%s", reply.GetAllowed(), reply.GetEffect())
}

func TestIntegrationRelationshipsWriteDelete(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMPermissionServiceClient(conn)
	ctx := adminContext()

// Write
		writeReply, err := client.WriteRelationships(ctx, &iamv1.WriteRelationshipsRequest{
			Relationships: []*iamv1.Relationship{
				{
					Resource: &iamv1.ObjectRef{Type: "zone", Id: "aisphere"},
					Relation: "member",
					Subject:  &iamv1.SubjectRef{Type: "user", Id: "test-int-user"},
				},
			},
		})
		if err != nil {
			t.Fatalf("WriteRelationships: %v", err)
		}
		t.Logf("WriteRelationships OK: written=%d", writeReply.GetWritten())

		// Read
		readReply, err := client.ReadRelationships(ctx, &iamv1.ListRelationshipsRequest{
			ResourceType: "zone",
			ResourceId:   "aisphere",
			Relation:     "member",
		})
		if err != nil {
			t.Fatalf("ReadRelationships: %v", err)
		}
		t.Logf("ReadRelationships OK: %d relationships", len(readReply.GetRelationships()))

		// Delete
		deleteReply, err := client.DeleteRelationships(ctx, &iamv1.DeleteRelationshipsRequest{
			Filter: &iamv1.RelationshipFilter{
				ResourceType: "zone",
				ResourceId:   "aisphere",
				Relation:     "member",
				SubjectType:  "user",
				SubjectId:    "test-int-user",
			},
		})
	if err != nil {
		t.Fatalf("DeleteRelationships: %v", err)
	}
	t.Logf("DeleteRelationships OK: deleted=%d", deleteReply.GetDeleted())
}

func TestIntegrationLookupResources(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMPermissionServiceClient(conn)

	reply, err := client.LookupResources(adminContext(), &iamv1.LookupResourcesRequest{
		Subject:      &iamv1.SubjectRef{Type: "user", Id: "admin"},
		ResourceType: "zone",
		Permission:   "view_zone",
	})
	if err != nil {
		t.Fatalf("LookupResources: %v", err)
	}
	t.Logf("LookupResources OK: %d results", len(reply.GetResources()))
}

// ── 5. AUTHZ-ADMIN: Authorization Admin ──

func TestIntegrationAdminGetSchema(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMAuthorizationAdminServiceClient(conn)

	schema, err := client.GetAuthorizationSchema(adminContext(), &iamv1.GetAuthorizationSchemaRequest{})
	if err != nil {
		t.Fatalf("GetAuthorizationSchema: %v", err)
	}
	if schema.GetVersion() == "" {
		t.Fatal("expected non-empty schema version")
	}
	t.Logf("GetAuthorizationSchema OK: version=%s, text length=%d", schema.GetVersion(), len(schema.GetText()))
}

func TestIntegrationAdminCheckPermission(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMAuthorizationAdminServiceClient(conn)

	reply, err := client.CheckAuthorization(adminContext(), &iamv1.CheckPermissionRequest{
		Subject:    &iamv1.SubjectRef{Type: "user", Id: "admin"},
		Resource:   &iamv1.ObjectRef{Type: "zone", Id: "aisphere"},
		Permission: "view_zone",
	})
	if err != nil {
		t.Fatalf("CheckAuthorization: %v", err)
	}
	t.Logf("CheckAuthorization OK: allowed=%v", reply.GetAllowed())
}

func TestIntegrationAdminExplain(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := iamv1.NewIAMAuthorizationAdminServiceClient(conn)

	reply, err := client.ExplainAuthorization(adminContext(), &iamv1.CheckPermissionRequest{
		Subject:    &iamv1.SubjectRef{Type: "user", Id: "admin"},
		Resource:   &iamv1.ObjectRef{Type: "zone", Id: "aisphere"},
		Permission: "view_zone",
	})
	if err != nil {
		t.Fatalf("ExplainAuthorization: %v", err)
	}
	t.Logf("ExplainAuthorization OK: allowed=%v, steps=%d", reply.GetAllowed(), len(reply.GetSteps()))
}

// ── 6. PROJECT: Project Lifecycle ──

func TestIntegrationProjectLifecycle(t *testing.T) {
	resources, cleanup := newResources(t)
	defer cleanup()

	projectID := "test-int-project-" + tsSuffix()
	project := &data.ProjectModel{
		ID:          projectID,
		OrgID:       "aisphere",
		Slug:        "test-int-" + tsSuffix(),
		DisplayName: "Test Integration Project",
		Status:      data.StatusActive,
	}
	if err := resources.ControlPlane.CreateProject(context.Background(), project); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	t.Log("CreateProject OK")

	got, err := resources.ControlPlane.GetProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Slug != project.Slug {
		t.Fatalf("unexpected slug: %s", got.Slug)
	}
	t.Log("GetProject OK")

got.DisplayName = "Updated Integration Project"
		if err := resources.ControlPlane.UpsertProject(context.Background(), got); err != nil {
			t.Fatalf("UpsertProject: %v", err)
		}
		t.Log("UpsertProject OK")

	if err := resources.ControlPlane.ArchiveProject(context.Background(), projectID); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	t.Log("ArchiveProject OK")
}

func TestIntegrationListCapabilities(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := projectv1.NewProjectServiceClient(conn)

	reply, err := client.ListCapabilities(adminContext(), &projectv1.ListCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("ListCapabilities: %v", err)
	}
	t.Logf("ListCapabilities OK: %d capabilities", len(reply.GetCapabilities()))
}

// ── 7. RESOURCE: Resource Lifecycle ──

func TestIntegrationResourceLifecycle(t *testing.T) {
	resources, cleanup := newResources(t)
	defer cleanup()

	rt := &data.ResourceTypeModel{
		Type:            "test_int_type",
		SpiceDBType:     "test_int_type",
		CapabilityID:    "iam",
		OwnerService:    "iam",
		RelationsJSON:   `["owner","viewer"]`,
		PermissionsJSON: `["read","write"]`,
		Auditable:       true,
	}
	if err := resources.ControlPlane.UpsertResourceType(context.Background(), rt); err != nil {
		t.Fatalf("UpsertResourceType: %v", err)
	}
	t.Log("UpsertResourceType OK")

	resourceID := "test-int-res-" + tsSuffix()
	resource := &data.ResourceModel{
		Type:           "test_int_type",
		ID:             resourceID,
		OrgID:          "aisphere",
		OwnerService:   "iam",
		OwnerResourceID: resourceID,
		Status:         data.StatusActive,
	}
	if err := resources.ControlPlane.UpsertResource(context.Background(), resource); err != nil {
		t.Fatalf("UpsertResource: %v", err)
	}
	t.Log("UpsertResource OK")

	got, err := resources.ControlPlane.GetResource(context.Background(), "test_int_type", resourceID)
	if err != nil {
		t.Fatalf("GetResource: %v", err)
	}
	if got.ID != resourceID {
		t.Fatalf("unexpected resource ID: %s", got.ID)
	}
	t.Log("GetResource OK")

	if err := resources.ControlPlane.ArchiveResource(context.Background(), "test_int_type", resourceID); err != nil {
		t.Fatalf("ArchiveResource: %v", err)
	}
	t.Log("ArchiveResource OK")

	if err := resources.ControlPlane.DeleteResource(context.Background(), "test_int_type", resourceID); err != nil {
		t.Fatalf("DeleteResource: %v", err)
	}
	t.Log("DeleteResource OK")
}

// ── 8. GRANT: Grant Lifecycle ──

func TestIntegrationGrantLifecycle(t *testing.T) {
	conn := newGRPCConn(t)
	defer conn.Close()
	client := grantv1.NewGrantServiceClient(conn)
	ctx := adminContext()

	// List role templates (defaults should be loaded)
	rtReply, err := client.ListRoleTemplates(ctx, &grantv1.ListRoleTemplatesRequest{})
	if err != nil {
		t.Fatalf("ListRoleTemplates: %v", err)
	}
	t.Logf("ListRoleTemplates OK: %d templates", len(rtReply.GetRoleTemplates()))

	// Register a role template
	rt, err := client.RegisterRoleTemplate(ctx, &grantv1.RegisterRoleTemplateRequest{
		RoleTemplate: &grantv1.RoleTemplate{
			ResourceType: "test_int_type",
			RoleKey:      "test-owner",
			Relation:     "owner",
			DisplayName:  "Test Owner",
		},
	})
	if err != nil {
		t.Fatalf("RegisterRoleTemplate: %v", err)
	}
	t.Logf("RegisterRoleTemplate OK: key=%s", rt.GetRoleKey())

// Grant access with expiry
		expiry := timestamppb.New(time.Now().Add(24 * time.Hour))
		grant, err := client.GrantAccess(ctx, &grantv1.GrantAccessRequest{
			Resource:  &resourcev1.ResourceRef{Type: "test_int_type", Id: "test-resource"},
			RoleKey:   "test-owner",
			Subject:   &resourcev1.SubjectRef{Type: "user", Id: "admin"},
			ExpiresAt: expiry,
		})
	if err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}
	t.Logf("GrantAccess OK: id=%s", grant.GetId())

	// List grants
	listReply, err := client.ListGrants(ctx, &grantv1.ListGrantsRequest{
		Resource: &resourcev1.ResourceRef{Type: "test_int_type", Id: "test-res"},
	})
	if err != nil {
		t.Fatalf("ListGrants: %v", err)
	}
	t.Logf("ListGrants OK: %d grants", len(listReply.GetGrants()))

	// Revoke access
	revokeReply, err := client.RevokeAccess(ctx, &grantv1.RevokeAccessRequest{
		GrantId: grant.GetId(),
	})
	if err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}
	if !revokeReply.GetRevoked() {
		t.Fatal("expected revoked=true")
	}
	t.Log("RevokeAccess OK")
}

// ── 9. AuthN Principal Context ──

func TestIntegrationAuthnPrincipal(t *testing.T) {
	principal := authn.Principal{
		SubjectID:   "test-user",
		SubjectType: authn.SubjectTypeUser,
		OrgID:       "aisphere",
		Provider:    "casdoor",
	}
	ctx := authn.ContextWithPrincipal(context.Background(), principal)

	got, ok := authn.PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("expected principal from context")
	}
	if got.SubjectID != "test-user" {
		t.Fatalf("unexpected subject: %s", got.SubjectID)
	}
	t.Log("Principal context test OK")
}

// ─────────────────────────────────────────────
// 10. 权限语义测试 — 直接权限增删验证
// ─────────────────────────────────────────────
//
// 这些测试通过 gRPC 调用 IAMAuthorizationAdminService 来操作 SpiceDB
// 关系并验证权限效果。admin 用户有 iam_authz:global#admin 权限。

// adminSvcClient 是 IAMAuthorizationAdminService 的 gRPC 客户端
func adminSvcClient(t *testing.T) iamv1.IAMAuthorizationAdminServiceClient {
	t.Helper()
	conn := newGRPCConn(t)
	t.Cleanup(func() { conn.Close() })
	return iamv1.NewIAMAuthorizationAdminServiceClient(conn)
}

func checkAuthz(t *testing.T, client iamv1.IAMAuthorizationAdminServiceClient, subType, subID, resType, resID, permission string) bool {
	t.Helper()
	reply, err := client.CheckAuthorization(adminContext(), &iamv1.CheckPermissionRequest{
		Subject:    &iamv1.SubjectRef{Type: subType, Id: subID},
		Resource:   &iamv1.ObjectRef{Type: resType, Id: resID},
		Permission: permission,
	})
	if err != nil {
		t.Fatalf("Check(%s:%s#%s on %s:%s): %v", subType, subID, permission, resType, resID, err)
	}
	return reply.GetAllowed()
}

func writeRel(t *testing.T, client iamv1.IAMAuthorizationAdminServiceClient, resType, resID, relation, subjType, subjID string) {
	t.Helper()
	_, err := client.WriteRelationships(adminContext(), &iamv1.WriteRelationshipsRequest{
		Relationships: []*iamv1.Relationship{
			{
				Resource: &iamv1.ObjectRef{Type: resType, Id: resID},
				Relation: relation,
				Subject:  &iamv1.SubjectRef{Type: subjType, Id: subjID},
			},
		},
	})
	if err != nil {
		t.Fatalf("Write(%s:%s#%s@%s:%s): %v", resType, resID, relation, subjType, subjID, err)
	}
}

func delRelPerm(client iamv1.IAMAuthorizationAdminServiceClient, resType, subID, relation, subType, subjID string) error {
	_, err := client.DeleteRelationships(adminContext(), &iamv1.DeleteRelationshipsRequest{
		Filter: &iamv1.RelationshipFilter{
			ResourceType: resType,
			ResourceId:   subID,
			Relation:     relation,
			SubjectType:  subType,
			SubjectId:    subjID,
		},
	})
	return err
}

// ── A1: Zone 权限授予与撤销 ──

func TestPermissionZoneGrantRevoke(t *testing.T) {
	c := adminSvcClient(t)
	user := "test-zone-user-" + tsSuffix()

	// 初始：无权限
	if checkAuthz(t, c, "user", user, "zone", "aisphere", "view_zone") {
		t.Fatal("expected denied before granting zone member")
	}

	// 授予 zone#member
	writeRel(t, c, "zone", "aisphere", "member", "user", user)
	t.Cleanup(func() { delRelPerm(c, "zone", "aisphere", "member", "user", user) })

	// 验证：有权限
	if !checkAuthz(t, c, "user", user, "zone", "aisphere", "view_zone") {
		t.Fatal("expected allowed after granting zone#member")
	}

	// 撤销
	if err := delRelPerm(c, "zone", "aisphere", "member", "user", user); err != nil {
		t.Fatalf("delete zone member: %v", err)
	}

	// 验证：无权限
	if checkAuthz(t, c, "user", user, "zone", "aisphere", "view_zone") {
		t.Fatal("expected denied after revoking zone#member")
	}
}

// ── A2: Group 权限继承 ──

func TestPermissionGroupMemberInheritsView(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-group-test-" + tsSuffix()
	groupID := "aisphere/test-group"

	// 初始：无权限
	if checkAuthz(t, c, "user", user, "group", groupID, "view") {
		t.Fatal("expected denied before group membership")
	}

	// 授予 group#member
	writeRel(t, c, "group", groupID, "member", "user", user)
	t.Cleanup(func() { delRelPerm(c, "group", groupID, "member", "user", user) })

	// 验证：group#member 自动获得 group#view
	if !checkAuthz(t, c, "user", user, "group", groupID, "view") {
		t.Fatal("expected allowed after group#member (member -> view)")
	}

	// 撤销
	if err := delRelPerm(c, "group", groupID, "member", "user", user); err != nil {
		t.Fatalf("delete group member: %v", err)
	}

	// 验证：无权限
	if checkAuthz(t, c, "user", user, "group", groupID, "view") {
		t.Fatal("expected denied after removing group#member")
	}
}

// ── A3: Project 权限 ──

func TestPermissionProjectViewerGrantsRead(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-proj-test-" + tsSuffix()
	projectID := "test-project"

	// 初始：无权限
	if checkAuthz(t, c, "user", user, "project", projectID, "read") {
		t.Fatal("expected denied before project#viewer")
	}

	// 授予 project#viewer
	writeRel(t, c, "project", projectID, "viewer", "user", user)
	t.Cleanup(func() { delRelPerm(c, "project", projectID, "viewer", "user", user) })

	// 验证：viewer 获得 read
	if !checkAuthz(t, c, "user", user, "project", projectID, "read") {
		t.Fatal("expected allowed after project#viewer (viewer -> read)")
	}

	// 撤销
	if err := delRelPerm(c, "project", projectID, "viewer", "user", user); err != nil {
		t.Fatalf("delete project viewer: %v", err)
	}

	// 验证：无权限
	if checkAuthz(t, c, "user", user, "project", projectID, "read") {
		t.Fatal("expected denied after removing project#viewer")
	}
}

// ── A4: Grant 授权与撤销 ──

func TestPermissionGrantAccessThenRevoke(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-grant-test-" + tsSuffix()
	resourceType := "test_int_type"
	resourceID := "test-grant-res"

	// 初始：无权限
	if checkAuthz(t, c, "user", user, resourceType, resourceID, "edit") {
		t.Fatal("expected denied before grant")
	}

	// 直接写 owner 关系（模拟 GrantAccess 的效果）
	writeRel(t, c, resourceType, resourceID, "owner", "user", user)
	t.Cleanup(func() { delRelPerm(c, resourceType, resourceID, "owner", "user", user) })

	// 验证：owner 获得 edit
	if !checkAuthz(t, c, "user", user, resourceType, resourceID, "edit") {
		t.Fatal("expected allowed after grant (owner -> edit)")
	}

	// 撤销
	if err := delRelPerm(c, resourceType, resourceID, "owner", "user", user); err != nil {
		t.Fatalf("delete owner: %v", err)
	}

	// 验证：无权限
	if checkAuthz(t, c, "user", user, resourceType, resourceID, "edit") {
		t.Fatal("expected denied after revoke")
	}
}

// ─────────────────────────────────────────────
// 11. 组与组关系测试
// ─────────────────────────────────────────────

// ── B1: 用户加入组获得权限 ──

func TestGroupJoinGrantsView(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-group-join-" + tsSuffix()
	groupID := "aisphere/test-group"

	// 初始：无权限
	if checkAuthz(t, c, "user", user, "group", groupID, "view") {
		t.Fatal("expected denied before joining group")
	}

	// 加入组
	writeRel(t, c, "group", groupID, "member", "user", user)
	t.Cleanup(func() { delRelPerm(c, "group", groupID, "member", "user", user) })

	// 验证：member 获得 view
	if !checkAuthz(t, c, "user", user, "group", groupID, "view") {
		t.Fatal("expected allowed after joining group (member -> view)")
	}
}

// ── B2: 用户移出组失去权限 ──

func TestPermissionLeaveGroupLosesView(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-group-leave-" + tsSuffix()
	groupID := "aisphere/test-group"

	// 先加入
	writeRel(t, c, "group", groupID, "member", "user", user)

	// 验证有权限
	if !checkAuthz(t, c, "user", user, "group", groupID, "view") {
		t.Fatal("expected allowed after joining group")
	}

	// 移出组
	if err := delRelPerm(c, "group", groupID, "member", "user", user); err != nil {
		t.Fatalf("delete group member: %v", err)
	}

	// 验证无权限
	if checkAuthz(t, c, "user", user, "group", groupID, "view") {
		t.Fatal("expected denied after leaving group")
	}
}

// ── B3: 子组继承父组权限 ──

func TestPermissionChildGroupInheritsParentView(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-child-group-" + tsSuffix()
	parentGroup := "aisphere/test-group"
	childGroup := "aisphere/test-child-group"

	// 创建子组关系：childGroup#parent@parentGroup
	writeRel(t, c, "group", childGroup, "parent", "group", parentGroup)
	t.Cleanup(func() { delRelPerm(c, "group", childGroup, "parent", "group", parentGroup) })

	// 给 user 分配 childGroup#member
	writeRel(t, c, "group", childGroup, "member", "user", user)
	t.Cleanup(func() { delRelPerm(c, "group", childGroup, "member", "user", user) })

	// 验证：childGroup#member 通过 parent->view 获得 parentGroup#view
	if !checkAuthz(t, c, "user", user, "group", parentGroup, "view") {
		t.Fatal("expected allowed: child member inherits parent view via parent->view")
	}
}

// ─────────────────────────────────────────────
// 12. 层级资源权限传递测试
// ─────────────────────────────────────────────

// ── C1: Project → SkillSpace 继承 ──

func TestPermissionProjectViewerInheritsToSkillSpace(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-proj-skill-" + tsSuffix()
	projectID := "test-project"
	skillSpaceID := "test-skill-space"

	// 给 user 分配 project#viewer
	writeRel(t, c, "project", projectID, "viewer", "user", user)
	t.Cleanup(func() { delRelPerm(c, "project", projectID, "viewer", "user", user) })

	// 建立 skill_space#parent@project
	writeRel(t, c, "skill_space", skillSpaceID, "parent", "project", projectID)
	t.Cleanup(func() { delRelPerm(c, "skill_space", skillSpaceID, "parent", "project", projectID) })

	// 验证：project#viewer → project#read → skill_space#parent->read → skill_space#view
	if !checkAuthz(t, c, "user", user, "skill_space", skillSpaceID, "view") {
		t.Fatal("expected allowed: project viewer inherits to skill_space view")
	}
}

// ── C2: SkillSpace → Skill 继承 ──

func TestPermissionSkillSpaceEditorInheritsToSkill(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-skill-edit-" + tsSuffix()
	skillSpaceID := "test-skill-space"
	skillID := "test-skill"

	// 给 user 在 skill_space#editor
	writeRel(t, c, "skill_space", skillSpaceID, "editor", "user", user)
	t.Cleanup(func() { delRelPerm(c, "skill_space", skillSpaceID, "editor", "user", user) })

	// 建立 skill#parent@skill_space
	writeRel(t, c, "skill", skillID, "parent", "skill_space", skillSpaceID)
	t.Cleanup(func() { delRelPerm(c, "skill", skillID, "parent", "skill_space", skillSpaceID) })

	// 验证：skill_space#editor → skill_space#edit → skill#parent#edit → skill#edit
	if !checkAuthz(t, c, "user", user, "skill", skillID, "edit") {
		t.Fatal("expected allowed: skill_space editor inherits edit to skill")
	}
}

// ── C3: Zone → Project 继承 ──

func TestPermissionZoneViewZoneInheritsToProjectRead(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-zone-proj-" + tsSuffix()
	projectID := "test-project"

	// 给 user 在 zone#member（通过 zone#member 获得 view_zone）
	writeRel(t, c, "zone", "aisphere", "member", "user", user)
	t.Cleanup(func() { delRelPerm(c, "zone", "aisphere", "member", "user", user) })

	// 验证：zone#member → zone#view_zone → project#zone → project#read
	if !checkAuthz(t, c, "user", user, "project", projectID, "read") {
		t.Fatal("expected allowed: zone member inherits project read via zone->view_zone")
	}
}

// ─────────────────────────────────────────────
// 13. 负面测试
// ─────────────────────────────────────────────

// ── D1: 无权限用户被拒绝 ──

func TestPermissionNoAccessUserDenied(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-eve-" + tsSuffix()

	// eve 没有任何权限，应该被拒绝
	if checkAuthz(t, c, "user", user, "zone", "aisphere", "view_zone") {
		t.Fatal("expected denied for user with no permissions")
	}
}

// ── D2: 错误权限被拒绝 ──

func TestPermissionWrongPermissionDenied(t *testing.T) {
	c := adminSvcClient(t)
	user := "user-wrong-perm-" + tsSuffix()

	// 给 user 添加 zone#member（只有 view_zone，没有 manage_users）
	writeRel(t, c, "zone", "aisphere", "member", "user", user)
	t.Cleanup(func() { delRelPerm(c, "zone", "aisphere", "member", "user", user) })

	// member 应该能 view_zone
	if !checkAuthz(t, c, "user", user, "zone", "aisphere", "view_zone") {
		t.Fatal("expected allowed: member can view_zone")
	}

	// member 不能 manage_users（需要 owner/admin/user_manager）
	if checkAuthz(t, c, "user", user, "zone", "aisphere", "manage_users") {
		t.Fatal("expected denied: member cannot manage_users")
	}
}