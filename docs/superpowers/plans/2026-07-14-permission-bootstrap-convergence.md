# Permission Bootstrap Convergence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Centralize IAM bootstrap policy in the permission manifest, prevent manifest/schema drift, and apply only strict additive SpiceDB schema changes during startup.

**Architecture:** Add a dependency-neutral internal/permissionmanifest package that loads the existing defaults file, parses the repository SpiceDB schema, validates catalog consistency, resolves bootstrap roles, and classifies schema changes. data.NewResources loads the manifest once, performs additive schema convergence, and writes bootstrap relationships from manifest policy; the business defaults reconciler consumes that same parsed manifest.

**Tech Stack:** Go 1.24, gopkg.in/yaml.v3, Kernel authz.SchemaManager and relationship contracts, Make, GitHub Actions.

---

## File structure

- Create internal/permissionmanifest/manifest.go for YAML types, loading, and bootstrap role lookup.
- Create internal/permissionmanifest/schema.go for structural parsing and additive-change classification.
- Create internal/permissionmanifest/validate.go for manifest/schema consistency checks.
- Create focused tests in internal/permissionmanifest and internal/data.
- Create cmd/permission-manifest-check/main.go for the offline repository gate.
- Extend configs/resource/defaults.yaml with bootstrap role and admin-resource policy.
- Modify data, defaults reconciliation, configuration, and main wiring to load one manifest.
- Modify Makefile, CI, Agile V records, and operator documentation.

### Task 1: Shared permission manifest

**Files:**
- Create: internal/permissionmanifest/manifest.go
- Create: internal/permissionmanifest/manifest_test.go
- Modify: configs/resource/defaults.yaml

- [ ] **Step 1: Write failing manifest tests**

Test the wished-for API:

    func TestLoadResolvesBootstrapRoleAliases(t *testing.T) {
        manifest, err := Load(filepath.Join("..", "..", "configs", "resource", "defaults.yaml"))
        if err != nil { t.Fatal(err) }
        role, canonical, ok := manifest.ResolveBootstrapRole("owner")
        if !ok || canonical != "zone_owner" || !role.ControlPlaneAdmin {
            t.Fatalf("resolved = %#v, %q, %v", role, canonical, ok)
        }
        want := []string{"owner", "admin", "user_manager", "group_manager", "permission_admin"}
        if !slices.Equal(want, role.ZoneRelations) { t.Fatalf("relations = %v", role.ZoneRelations) }
    }

    func TestResolveBootstrapRoleUsesConfiguredDefault(t *testing.T) {
        manifest := Manifest{Bootstrap: BootstrapPolicy{
            DefaultRole: "zone_owner",
            Roles: map[string]BootstrapRole{"zone_owner": {ZoneRelations: []string{"owner"}}},
        }}
        _, canonical, ok := manifest.ResolveBootstrapRole("")
        if !ok || canonical != "zone_owner" { t.Fatalf("canonical = %q, ok = %v", canonical, ok) }
    }

- [ ] **Step 2: Verify RED**

Run: go test ./internal/permissionmanifest -run 'TestLoad|TestResolve' -count=1

Expected: FAIL because the package and API do not exist.

- [ ] **Step 3: Implement the manifest package**

Define Manifest with Capabilities, ResourceTypes, RoleTemplates, and Bootstrap. Define BootstrapPolicy with DefaultRole, Roles, and AdminResources. Define BootstrapRole with Aliases, ZoneRelations, and ControlPlaneAdmin. Define AdminResource with Type and ID. Load reads YAML through yaml.Unmarshal. ResolveBootstrapRole trims input, uses DefaultRole for empty input, and resolves canonical keys or aliases.

Extend defaults.yaml with explicit policies for zone_owner, zone_admin, user_viewer, user_manager, group_viewer, group_manager, and permission_admin. Add the nine resources currently returned by defaultControlPlaneAdminResources.

- [ ] **Step 4: Verify GREEN**

Run: go test ./internal/permissionmanifest -run 'TestLoad|TestResolve' -count=1

Expected: PASS.

- [ ] **Step 5: Commit**

    git add -- internal/permissionmanifest/manifest.go internal/permissionmanifest/manifest_test.go configs/resource/defaults.yaml
    git commit -m "feat: centralize IAM bootstrap permission policy"

### Task 2: Schema parser and consistency model

**Files:**
- Create: internal/permissionmanifest/schema.go
- Create: internal/permissionmanifest/schema_test.go
- Create: internal/permissionmanifest/validate.go
- Modify: internal/permissionmanifest/manifest_test.go

- [ ] **Step 1: Write failing classification tests**

Add table-driven tests for:

    current: definition user {} definition zone { relation owner: user }
    desired: definition user {} definition zone { relation owner: user permission view = owner }
    want addition: zone.permission.view

    current: permission view = owner
    desired: permission view = owner + admin
    want conflict: zone.permission.view changed

    current contains definition legacy, desired does not
    want conflict: definition legacy exists only in active schema

Also test comment/whitespace normalization and duplicate definition/member errors.

- [ ] **Step 2: Verify RED**

Run: go test ./internal/permissionmanifest -run 'TestParseSchema|TestCompareSchemas' -count=1

Expected: FAIL because ParseSchema and CompareSchemas are undefined.

- [ ] **Step 3: Implement structural parsing**

Implement Schema, Definition, and SchemaDiff types plus:

    func ParseSchema(text string) (Schema, error)
    func CompareSchemas(current, desired Schema) SchemaDiff
    func (d SchemaDiff) Identical() bool
    func (d SchemaDiff) Additive() bool

Remove line/block comments, find balanced definition blocks, extract relation and permission declarations, normalize expression whitespace, sort diagnostics, reject active-only or changed declarations, and report desired-only declarations as additions.

- [ ] **Step 4: Write the failing committed-file consistency test**

Load configs/resource/defaults.yaml and configs/spicedb/aisphere.schema.zed, parse both, and call the wished-for Validate function. Expect no error for committed files. Add negative fixtures for missing permission, invalid role relation, duplicate alias, unknown bootstrap resource, and unresolved default role.

- [ ] **Step 5: Verify RED, implement validation, verify GREEN**

Run before implementation: go test ./internal/permissionmanifest -run 'TestCommitted|TestValidate' -count=1

Expected: FAIL because Validate is undefined.

Validate must exactly compare each catalog resource's relation/permission sets with its spicedb_type, verify role-template relations, validate bootstrap zone relations/admin-resource definitions, reject duplicate aliases, and require a resolvable default role.

Run after implementation: go test ./internal/permissionmanifest -count=1

Expected: PASS.

- [ ] **Step 6: Commit**

    git add -- internal/permissionmanifest
    git commit -m "feat: validate permission manifest against SpiceDB schema"

### Task 3: Additive-only schema bootstrap

**Files:**
- Create: internal/data/authz_bootstrap_test.go
- Modify: internal/data/authz_bootstrap.go

- [ ] **Step 1: Write failing bootstrap tests**

Create a fake authz.SchemaManager and test:

    TestBootstrapAuthzSchemaSkipsIdenticalSchema
    TestBootstrapAuthzSchemaPublishesMissingPermission
    TestBootstrapAuthzSchemaRejectsChangedPermission
    TestBootstrapAuthzSchemaRejectsActiveOnlyDefinition
    TestBootstrapAuthzSchemaFailsClosedWhenReadFails

Each test writes desired schema to t.TempDir, passes the path in conf.AuthzConfig, and asserts validation/write counts and error diagnostics.

- [ ] **Step 2: Verify RED**

Run: go test ./internal/data -run TestBootstrapAuthzSchema -count=1

Expected: FAIL because the four-definition probe skips missing permissions and attempts writes after read errors.

- [ ] **Step 3: Implement additive convergence**

Narrow the function to:

    func BootstrapAuthzSchema(ctx context.Context, cfg conf.AuthzConfig, manager authz.SchemaManager, log logx.Logger) error

Load/parse desired schema, read/parse active schema, compare through permissionmanifest.CompareSchemas, skip identical declarations, fail closed with sorted conflicts, and call ValidateSchema then WriteSchema only for an additive diff. Remove schemaReady.

- [ ] **Step 4: Verify GREEN and commit**

Run: go test ./internal/data ./internal/permissionmanifest -count=1

Expected: PASS.

    git add -- internal/data/authz_bootstrap.go internal/data/authz_bootstrap_test.go
    git commit -m "fix: apply only additive SpiceDB schema changes"

### Task 4: Manifest-driven administrator bootstrap and one-time loading

**Files:**
- Create: internal/data/bootstrap_admin_test.go
- Modify: internal/data/data.go
- Modify: internal/conf/conf.go
- Modify: internal/biz/defaults/loader.go
- Modify: cmd/aisphere-iam/main.go

- [ ] **Step 1: Write failing administrator tests**

Use authz.NewMemoryRelationshipStore with direct subjects. Provide a small BootstrapPolicy and assert zone owner/permission_admin plus iam_authz:global admin relationships. Add a test proving an unknown configured role returns an error rather than becoming an arbitrary relation.

- [ ] **Step 2: Verify RED**

Run: go test ./internal/data -run TestBootstrapControlPlaneAdmins -count=1

Expected: FAIL because the function does not accept manifest policy and unknown roles currently pass through.

- [ ] **Step 3: Implement manifest-driven wiring**

Add PermissionManifest *permissionmanifest.Manifest to Resources. NewResources loads control_plane.defaults.path once whenever defaults or bootstrap admins are enabled, parses security.authz.schema_path, and calls permissionmanifest.Validate before any schema or relationship write. Pass its Bootstrap policy to bootstrapControlPlaneAdmins and retain the parsed manifest for reconciliation.

Delete bootstrapRoleToRelation, bootstrapZoneRelations, bootstrapRoleGrantsControlPlaneAdmin, and defaultControlPlaneAdminResources. Remove the environment-level Resources override and ControlPlaneAdminResource type from conf.go.

Replace duplicated defaults loader structs with shared permissionmanifest types. Change main from ReconcileFile to Reconcile using resources.PermissionManifest.

- [ ] **Step 4: Verify GREEN and commit**

Run: go test ./internal/data ./internal/biz/defaults ./cmd/aisphere-iam -count=1

Expected: PASS.

    git add -- internal/data internal/conf/conf.go internal/biz/defaults/loader.go cmd/aisphere-iam/main.go
    git commit -m "refactor: drive IAM bootstrap grants from permission manifest"

### Task 5: Required repository gate and Agile V traceability

**Files:**
- Create: cmd/permission-manifest-check/main.go
- Modify: Makefile
- Modify: .github/workflows/ci.yml
- Modify: .agile-v/requirements/requirements.md
- Modify: .agile-v/BUILD_MANIFEST.md
- Modify: .agile-v/TEST_SPEC.md

- [ ] **Step 1: Implement the offline command**

Accept --manifest and --schema flags, load/parse/validate through permissionmanifest, and print deterministic counts:

    permission manifest valid: 15 resource types, 24 role templates, 20 schema definitions

- [ ] **Step 2: Prove drift detection**

Run the command against committed files and expect success. Run a validator test against a temporary manifest missing one permission and expect an error naming the resource and missing permission.

- [ ] **Step 3: Wire Make and CI**

Add PERMISSION_MANIFEST, a permission-manifest-check target, help text, and the target to verify. Run it in CI before proto-check.

- [ ] **Step 4: Update traceability**

Add REQ-IAM-ENG-009 for deterministic permission bootstrap/drift prevention, ART-0063 records for permissionmanifest, authz_bootstrap, and the command gate, and TC-0055 records for consistency/additive bootstrap tests. Update summary counts.

- [ ] **Step 5: Run gates and commit**

Run:

    make permission-manifest-check
    make traceability-check STRICT=1

Expected: both exit 0.

    git add -- cmd/permission-manifest-check Makefile .github/workflows/ci.yml .agile-v
    git commit -m "ci: require IAM permission manifest consistency"

### Task 6: Documentation and final verification

**Files:**
- Modify: configs/spicedb/README.md
- Modify: docs/authz-bootstrap-and-permission-console.md
- Modify: docs/iam-authz-spicedb-bootstrap.md

- [ ] **Step 1: Update operator documentation**

Document manifest ownership, .zed executable semantics, the consistency gate, automatic strict additions, and explicit migrations for changes/removals.

- [ ] **Step 2: Run focused tests**

Run: go test ./internal/permissionmanifest ./internal/data ./internal/biz/defaults -count=1

Expected: PASS.

- [ ] **Step 3: Run repository verification**

Run each command separately:

    go test ./... -count=1
    go build ./cmd/aisphere-iam
    make permission-manifest-check
    make proto-check
    make traceability-check STRICT=1
    git diff --check

Expected: every command exits 0. Record exact unrelated environment failures separately from touched-package failures.

- [ ] **Step 4: Commit documentation**

    git add -- configs/spicedb/README.md docs/authz-bootstrap-and-permission-console.md docs/iam-authz-spicedb-bootstrap.md
    git commit -m "docs: document additive IAM permission bootstrap"

- [ ] **Step 5: Review final scope**

Run git status --short, git log --oneline -8, and git diff HEAD~5 --stat. Confirm only planned permission-bootstrap, gate, traceability, and documentation files changed.
