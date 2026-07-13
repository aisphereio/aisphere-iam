from pathlib import Path
import textwrap

proto_path = Path("api/iam/v1/iam.proto")
text = proto_path.read_text()

policy_replacements = {
    'authz: { action: "read" resource: "iam:org:{org_id}:user:{user_id}" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "view_users" resource: "zone:{org_id}" audience: "iam-service" mode: SELF_CHECK }',
    'authz: { action: "list" resource: "iam:org:{org_id}:user:*" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "view_users" resource: "zone:{org_id}" audience: "iam-service" mode: SELF_CHECK }',
    'authz: { action: "read" resource: "iam:org:{org_id}:organization:{org_id}" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "view_zone" resource: "zone:{org_id}" audience: "iam-service" mode: SELF_CHECK }',
    'authz: { action: "list" resource: "iam:org:{org_id}:group:*" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "view_groups" resource: "zone:{org_id}" audience: "iam-service" mode: SELF_CHECK }',
    'authz: { action: "view" resource: "group:{group_id}" audience: "iam-service" mode: SELF_CHECK }':
        'authz: { action: "view_groups" resource: "zone:{org_id}" audience: "iam-service" mode: SELF_CHECK }',
}
for old, new in policy_replacements.items():
    if old not in text:
        raise SystemExit(f"missing expected policy: {old}")
    text = text.replace(old, new, 1)

directory_start = text.index("service IAMDirectoryService {")
group_write_start = text.index("  rpc CreateGroup(", directory_start)
directory_end = text.index("}\n\nservice IAMDirectoryProjectionService", group_write_start)
text = text[:group_write_start] + text[directory_end:]

permission_start = text.index("service IAMPermissionService {")
singular_start = text.index("  rpc WriteRelationship(", permission_start)
lookup_start = text.index("  rpc LookupResources(", singular_start)
text = text[:singular_start] + text[lookup_start:]
proto_path.write_text(text)

identity_mode = Path("internal/data/identity_mode.go")
identity_text = identity_mode.read_text()
old_filter = 'ResourceType: "group", ResourceID: strings.TrimSpace(req.GroupID), Relation: "member", SubjectType: "user", SubjectID: strings.TrimSpace(req.UserID)'
new_filter = 'ResourceType: "group", ResourceID: qualifiedGroupID(req.OrgID, req.GroupID), Relation: "member", SubjectType: "user", SubjectID: strings.TrimSpace(req.UserID)'
if old_filter not in identity_text:
    raise SystemExit("missing unqualified membership delete filter")
identity_mode.write_text(identity_text.replace(old_filter, new_filter, 1))

Path("internal/service/directory_group.go").write_text(textwrap.dedent("""\
    package service

    import (
        "context"

        v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
        "github.com/aisphereio/kernel/authn"
    )

    // GetGroup reads one Casdoor-backed group after applying the same
    // zone-scoped directory authorization contract used by ListGroups.
    func (s *IAMDirectoryService) GetGroup(ctx context.Context, req *v1.GetGroupRequest) (*v1.Group, error) {
        if err := s.requireZonePermission(ctx, req.GetOrgId(), "view_groups"); err != nil {
            return nil, err
        }
        if s.deps.Identity == nil {
            return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
        }
        group, err := s.deps.Identity.GetGroup(ctx, req.GetOrgId(), req.GetGroupId())
        if err != nil {
            return nil, err
        }
        return groupToProto(group), nil
    }
"""))

Path("internal/service/api_surface_contract_test.go").write_text(textwrap.dedent("""\
    package service

    import (
        "os"
        "path/filepath"
        "strings"
        "testing"
    )

    func TestIAMP0APIBoundaries(t *testing.T) {
        t.Parallel()

        root := filepath.Join("..", "..")
        iamProto := readAPISurfaceContract(t, filepath.Join(root, "api", "iam", "v1", "iam.proto"))
        identityProto := readAPISurfaceContract(t, filepath.Join(root, "api", "iam", "v1", "identity_admin.proto"))

        directory := protoServiceBlock(t, iamProto, "IAMDirectoryService")
        for _, forbidden := range []string{
            "rpc CreateGroup(",
            "rpc UpdateGroup(",
            "rpc DeleteGroup(",
            "rpc AssignUserToGroup(",
            "rpc RemoveUserFromGroup(",
        } {
            if strings.Contains(directory, forbidden) {
                t.Fatalf("directory read service contains write RPC %q", forbidden)
            }
        }
        for _, required := range []string{"rpc GetUser(", "rpc ListUsers(", "rpc GetOrganization(", "rpc ListGroups(", "rpc GetGroup("} {
            if !strings.Contains(directory, required) {
                t.Fatalf("directory service is missing read RPC %q", required)
            }
        }
        if strings.Contains(directory, `resource: "iam:org:`) {
            t.Fatal("directory policies still reference removed iam:org authorization resources")
        }

        permission := protoServiceBlock(t, iamProto, "IAMPermissionService")
        for _, forbidden := range []string{"rpc WriteRelationship(", "rpc DeleteRelationship("} {
            if strings.Contains(permission, forbidden) {
                t.Fatalf("runtime permission service exposes singular raw tuple RPC %q", forbidden)
            }
        }
        for _, required := range []string{"rpc WriteRelationships(", "rpc DeleteRelationships(", "rpc ReadRelationships("} {
            if !strings.Contains(permission, required) {
                t.Fatalf("runtime permission service is missing internal projection RPC %q", required)
            }
        }

        identityAdmin := protoServiceBlock(t, identityProto, "IAMIdentityAdminService")
        for _, required := range []string{
            "rpc CreateGroup(",
            "rpc UpdateGroup(",
            "rpc DeleteGroup(",
            "rpc AssignUserToGroup(",
            "rpc RemoveUserFromGroup(",
            `resource: "group:{org_id}/{group_id}"`,
        } {
            if !strings.Contains(identityAdmin, required) {
                t.Fatalf("identity admin is missing canonical group write contract %q", required)
            }
        }
    }

    func readAPISurfaceContract(t *testing.T, path string) string {
        t.Helper()
        data, err := os.ReadFile(path)
        if err != nil {
            t.Fatalf("read %s: %v", path, err)
        }
        return string(data)
    }

    func protoServiceBlock(t *testing.T, source, service string) string {
        t.Helper()
        marker := "service " + service + " {"
        start := strings.Index(source, marker)
        if start < 0 {
            t.Fatalf("service %s not found", service)
        }
        depth := 0
        for offset, r := range source[start:] {
            switch r {
            case '{':
                depth++
            case '}':
                depth--
                if depth == 0 {
                    return source[start : start+offset+1]
                }
            }
        }
        t.Fatalf("service %s has no closing brace", service)
        return ""
    }
"""))

Path("internal/data/identity_membership_projection_test.go").write_text(textwrap.dedent("""\
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
"""))

Path(".agile-v/change_requests/CR-0002-converge-iam-api-boundaries.md").write_text(textwrap.dedent("""\
    # CR-0002 — Converge IAM API Security Boundaries

    ## Status

    `IMPLEMENTED_PENDING_GATE_2 [C2]`

    ## Approved Gate 1 decisions

    1. Casdoor Organization → SpiceDB Zone is the single root model; PR #40 closed the legacy Organization control-plane gap.
    2. `IAMDirectoryService` is the read-only directory facade.
    3. `IAMIdentityAdminService` is the canonical User/Group mutation surface.
    4. Group authorization resources use Organization-qualified IDs: `group:<org_id>/<group_id>`.
    5. Raw Relationship mutation is not a product-facing capability. Runtime projection keeps INTERNAL plural APIs; administrative repair remains under `IAMAuthorizationAdminService` with global repair permission.

    ## Changes

    - remove Group and membership writes from `IAMDirectoryService`;
    - implement the missing read-only `GetGroup` adapter;
    - align directory read policies to Zone permissions and service-level checks;
    - remove AUTHORIZED singular Relationship write/delete RPCs;
    - retain INTERNAL plural relationship projection APIs;
    - fix membership removal to delete the Organization-qualified Group relationship;
    - add regression and projection tests;
    - regenerate Proto, transport, Gateway and deployment artifacts.

    ## Acceptance criteria

    - generated public routes contain one Group write surface only;
    - no public/authorized singular raw tuple mutation route exists;
    - directory read policies contain no removed `iam:org:*` resource;
    - removing membership deletes `group:<org_id>/<group_id>#member@user:<id>`;
    - `make proto-check`, `go test ./...` and `make build` pass;
    - generated-file drift is clean.

    ## Remaining Gate 2 work

    Real Casdoor, SpiceDB, PostgreSQL and Envoy Gateway integration evidence remains required before release readiness.
"""))
