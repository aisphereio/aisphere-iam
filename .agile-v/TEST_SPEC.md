# Test Specification — Aisphere IAM

> Cycle: C1 | Generated: 2026-07-13 | Status: COMPLETE

## 1. Existing Test Coverage

| TC-ID | REQ-ID | Description | Type | File | Status |
|-------|--------|-------------|------|------|--------|
| TC-0001 | AUTHN-001 | Principal extraction from Context | unit | `principal_context_test.go` | ✅ |
| TC-0002 | AUTHN-003 | Actor/owner fallback from Principal | unit | `principal_context_test.go` | ✅ |
| TC-0003 | PROJ-001 | Group identity projection shape | unit | `identity_mode_test.go` | ✅ |
| TC-0004 | PROJ-002 | Child Group parent relation | unit | `identity_mode_test.go` | ✅ |
| TC-0005 | PROJ-003 | User membership relationship | unit | `identity_mode_test.go` | ✅ |
| TC-0006 | AUTHZ-RT-002 | Batch check adapter | unit | `client/authzgrpc/client_test.go` | ✅ |
| TC-0007 | AUTHZ-RT-003 | Batch write adapter | unit | `client/authzgrpc/client_test.go` | ✅ |
| TC-0008 | AUTHZ-RT-005 | Relationship read adapter | unit | `client/authzgrpc/client_test.go` | ✅ |
| TC-0009 | AUTHZ-RT-008 | UUID propagation and Unicode profile metadata exclusion | unit | `client/authzgrpc/client_test.go` | ✅ |
| TC-0010 | PROJECT-001 | Single-root Organization model | contract | `model_contract_test.go` | ✅ |
| TC-0011 | PROJECT-002 | Project scope from Principal | unit | `principal_context_test.go` | ✅ |
| TC-0012 | PROJECT-003 | Creator ownership from Principal | unit | `principal_context_test.go` | ✅ |
| TC-0013 | ENG-001 | Proto generation drift check | CI | CI workflow | ✅ |
| TC-0014 | ENG-002 | Kernel version alignment | CI | CI workflow | ✅ |
| TC-0015 | ENG-003 | Contract check | CI | CI workflow | ✅ |
| TC-0016 | GRANT-001 | Role Template register/list | unit | `grant_test.go` | ✅ |
| TC-0017 | GRANT-002~003 | Grant and Revoke access | unit | `grant_test.go` | ✅ |
| TC-0018 | GRANT-005 | Explain access | unit | `grant_test.go` | ✅ |
| TC-0019 | GRANT-006 | Grant expiry executor | unit | `grant_test.go` | ✅ |
| TC-0020 | GRANT-006 | Grant expiry — no expiry grants | unit | `grant_test.go` | ✅ |
| TC-0021 | DIR-005 | Group CRUD via IAMGroupAdminService | unit | `group_admin_test.go` | ✅ |
| TC-0022 | DIR-006 | User membership assign/remove | unit | `group_admin_test.go` | ✅ |
| TC-0063 | DIR-005 | Persist and restore Group machine-name metadata across Casdoor round trips | unit | `internal/data/group_metadata_test.go` | ✅ |
| TC-0064 | DIR-004 | Resolve legacy User aliases and provider-side Group members to stable Group IDs | unit | `internal/service/directory_test.go` | ✅ |
| TC-0023 | PROJECT-001~008 | Project full lifecycle | unit | `project_lifecycle_test.go` | ✅ |
| TC-0024 | RESOURCE-001~007 | Resource full lifecycle | unit | `resource_lifecycle_test.go` | ✅ |
| TC-0025 | RESOURCE-006 | Bind/Unbind Resource | unit | `resource_lifecycle_test.go` | ✅ |
| TC-0026 | RESOURCE-007 | External Resource bindings | unit | `resource_lifecycle_test.go` | ✅ |
| TC-0027 | AUTHZ-ADMIN-001 | Get schema | unit | `authz_admin_test.go` | ✅ |
| TC-0028 | AUTHZ-ADMIN-002 | Validate schema | unit | `authz_admin_test.go` | ✅ |
| TC-0029 | AUTHZ-ADMIN-003 | Check permission | unit | `authz_admin_test.go` | ✅ |
| TC-0030 | AUTHZ-ADMIN-004 | Explain authorization | unit | `authz_admin_test.go` | ✅ |
| TC-0031 | AUTHZ-ADMIN-004 | Get effective permissions | unit | `authz_admin_test.go` | ✅ |
| TC-0055 | ENG-009 | Committed permission manifest matches SpiceDB schema | unit | `internal/permissionmanifest/manifest_test.go` | ✅ |
| TC-0056 | ENG-009 | Schema bootstrap applies additions and rejects changes/removals | unit | `internal/data/authz_bootstrap_test.go` | ✅ |
| TC-0057 | GRANT-007 | Scoped bootstrap aliases, manifest validation and schema alignment | unit | `internal/permissionmanifest/manifest_test.go` | ✅ |
| TC-0058 | GRANT-007 | Administrator relationship cardinality, migration cleanup and organization root projection | unit | `internal/data/bootstrap_admin_test.go`, `internal/data/identity_mode_test.go` | ✅ |
| TC-0059 | GRANT-007 | Batch grant deletion and custom role capability replacement | unit | `internal/biz/projection/manager_test.go` | ✅ |
| TC-0060 | GRANT-007 | Custom role registration, versioned update, impact preview, binding grant and revoke | unit | `internal/service/grant_test.go` | ✅ |
| TC-0061 | GRANT-007 | Every grantable schema resource exposes dynamic role bindings | unit | `internal/permissionmanifest/manifest_test.go` | ✅ |
| TC-0062 | GRANT-007 | Permission-expression migration requires an explicit flag and still rejects removals | unit | `internal/data/authz_bootstrap_test.go` | ✅ |
| TC-0065 | RESOURCE-008 | Skill is the sole Git authorization resource and removed Git resource types are not reconciled | contract | `internal/permissionmanifest/manifest_test.go`, `internal/data/identity_mode_test.go`, `cmd/permission-manifest-check/main_test.go` | ✅ |

## 2. Missing Test Coverage (P0 Priority)

| TC-ID | REQ-ID | Description | Type | Priority | Status |
|-------|--------|-------------|------|:--------:|--------|
| TC-0032 | AUTHN-002 | Casdoor token verification | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationAuthnVerifyToken` |
| TC-0033 | AUTHN-004 | Gateway trust boundary | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationAuthnGetMe` |
| TC-0034 | DIR-001 | User read with Zone permission | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationDirectoryGetUser` |
| TC-0035 | DIR-002 | User list with Zone permission | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationDirectoryListUsers` |
| TC-0036 | DIR-003 | Organization metadata read | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationDirectoryGetOrganization` |
| TC-0037 | DIR-004 | Group list with Zone permission | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationDirectoryListGroups` |
| TC-0038 | DIR-007 | Identity mode matrix | integration | P0 | ❌ |
| TC-0039 | AUTHZ-RT-001 | SpiceDB permission check | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationPermissionCheck` |
| TC-0040 | AUTHZ-RT-003 | Relationship write | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationRelationshipsWriteDelete` |
| TC-0041 | AUTHZ-RT-004 | Relationship delete | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationRelationshipsWriteDelete` |
| TC-0042 | AUTHZ-RT-006 | Resource lookup | integration | P0 | ✅ `iam_integration_test.go::TestIntegrationLookupResources` |
| TC-0043 | AUTHZ-RT-007 | Subject lookup | integration | P0 | ❌ |

## 3. Missing Test Coverage (P1 Priority)

| TC-ID | REQ-ID | Description | Type | Priority | Status |
|-------|--------|-------------|------|:--------:|--------|
| TC-0044 | PROJ-004 | Projection event persistence | integration | P1 | ❌ |
| TC-0045 | PROJ-005 | Projection retry | integration | P1 | ❌ |
| TC-0046 | PROJ-006 | DTM Saga | integration | P1 | ❌ |
| TC-0047 | PROJ-007 | Drift detection | integration | P1 | ❌ |
| TC-0048 | AUTHZ-ADMIN-001~005 | Admin operations | integration | P1 | ✅ `iam_integration_test.go::TestIntegrationAdminGetSchema, TestIntegrationAdminCheckPermission, TestIntegrationAdminExplain` |
| TC-0049 | GRANT-001~006 | Grant lifecycle | integration | P1 | ✅ `iam_integration_test.go::TestIntegrationGrantLifecycle` |

## 4. Missing Test Coverage (P2 Priority)

| TC ID | REQ-ID | Description | Type | Priority |
|-------|--------|-------------|------|:--------:|
| TC-0050 | ENG-004 | Fail-closed fault injection | integration | P2 |
| TC-0051 | ENG-005 | Error matrix | integration | P2 |
| TC-0052 | ENG-006 | Audit persistence | integration | P2 |
| TC-0053 | ENG-007 | Observability | integration | P2 |
| TC-0054 | ENG-008 | Release gate | integration | P2 |
| TC-0066 | PROJECT-004 | ListProjects contract requires `view_zone` on the requested Zone | contract | P0 | ✅ `api/iam/project/v1/project_policy_test.go` |

## 5. Summary

| Dimension | Count |
|-----------|:-----:|
| Existing test cases | 33 (unit) + 14 (integration) |
| Missing test cases (P0) | 2 (DIR-007, AUTHZ-RT-007) |
| Missing test cases (P1) | 4 (PROJ-004~007) |
| Missing test cases (P2) | 5 (ENG-004~008) |
