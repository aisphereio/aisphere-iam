# Testing

## Test strategy

IAM tests focus on:
1. **Service-layer unit tests** — Business logic with mocked dependencies
2. **Access control contract tests** — Generated authz policy verification
3. **Identity projection tests** — Casdoor → SpiceDB relationship correctness
4. **Skip policy tests** — Middleware skip-policy resolver correctness

## Test files

| File | What it tests |
|---|---|
| `internal/service/iam_test.go` | `IAMAuthService.GetMe`, `IAMPermissionService` check/write/delete relationships |
| `internal/server/access_test.go` | Skip policy resolver for IAMPermissionService operations |
| `internal/server/identity_admin_contract_test.go` | Generated access policies for identity admin operations |
| `internal/data/identity_mode_test.go` | Identity projection: zone-qualified group edges, user-group membership |
| `client/authzgrpc/client_test.go` | gRPC client conversion and error handling |

## Running tests

```bash
# All tests
make test

# Or directly
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Test patterns

### Service tests (`internal/service/iam_test.go`)

Use `authz.NewMemoryRelationshipStore()` for in-memory SpiceDB simulation:

```go
func TestIAMPermissionServiceWritesAndChecksRelationship(t *testing.T) {
    store := authz.NewMemoryRelationshipStore()
    admin := memoryAuthzAdmin{MemoryRelationshipStore: store}
    svc := NewIAMPermissionService(IAMDeps{Authz: admin})

    // Write relationship
    svc.WriteRelationship(ctx, &v1.WriteRelationshipRequest{...})

    // Check permission
    reply, err := svc.CheckPermission(ctx, &v1.CheckPermissionRequest{...})
}
```

### Access control contract tests (`internal/server/identity_admin_contract_test.go`)

Verify that generated authz policies map to the correct SpiceDB resource/permission:

```go
func TestIdentityAdminGeneratedAccessUsesGroupManagementModel(t *testing.T) {
    catalog := IAMCatalog()
    tests := []struct {
        operation  string
        resource   authz.ObjectRef
        permission string
    }{
        {
            operation:  "/iam.v1.IAMIdentityAdminService/CreateGroup",
            resource:   authz.ObjectRef{Type: "zone", ID: "aisphere"},
            permission: "create_groups",
        },
        {
            operation:  "/iam.v1.IAMIdentityAdminService/UpdateGroup",
            resource:   authz.ObjectRef{Type: "group", ID: "aisphere/platform"},
            permission: "manage",
        },
    }
    // ...
}
```

### Identity projection tests (`internal/data/identity_mode_test.go`)

Verify that identity mutations produce the correct SpiceDB relationships:

```go
func TestAuthzProjectingIdentityAdminWritesZoneQualifiedGroupEdges(t *testing.T) {
    store := authz.NewMemoryRelationshipStore()
    admin := BindIdentityAuthZ(fakeIdentityAdmin{...}, store)

    admin.CreateGroup(ctx, authn.CreateGroupRequest{...})
    admin.AssignUserToGroup(ctx, authn.AssignUserToGroupRequest{...})

    // Verify expected relationships exist
    rels, _ := store.ReadRelationships(ctx, authz.RelationshipFilter{})
    // Check: group:aisphere/platform#zone@zone:aisphere
    // Check: group:aisphere/platform#member@user:user-1
}
```

### Skip policy tests (`internal/server/access_test.go`)

Verify that the skip policy resolver correctly skips authz for IAMPermissionService operations (to prevent recursive authorization checks):

```go
func TestIAMInternalPermissionRPCSkipsRecursiveResourceCheck(t *testing.T) {
    resolver := iamSkipPolicyResolver(IAMCatalog())
    operations := []string{
        "/iam.v1.IAMPermissionService/CheckPermission",
        "/iam.v1.IAMPermissionService/BatchCheckPermissions",
        // ...
    }
    for _, op := range operations {
        if got := resolver(op); got != accessx.SkipAuthz {
            t.Errorf("%s skip policy = %v, want SkipAuthz", op, got)
        }
    }
}
```

## What to test when changing

| Change area | Tests to run |
|---|---|
| Proto contract change | `make api && make proto-check && make test` |
| New RPC | Add contract test in `identity_admin_contract_test.go` |
| Access policy change | Update skip policy test in `access_test.go` |
| Identity projection | Run `identity_mode_test.go` |
| Service logic | Run `iam_test.go` |
| gRPC client | Run `client/authzgrpc/client_test.go` |

## Key source files

| File | Purpose |
|---|---|
| `internal/service/iam_test.go` | Service-layer tests |
| `internal/server/access_test.go` | Skip policy resolver tests |
| `internal/server/identity_admin_contract_test.go` | Authz policy contract tests |
| `internal/data/identity_mode_test.go` | Identity projection tests |
| `client/authzgrpc/client_test.go` | gRPC client tests |