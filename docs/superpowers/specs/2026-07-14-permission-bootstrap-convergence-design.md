# Permission Bootstrap Convergence Design

## Goal

Make IAM permission initialization deterministic without treating every service restart as an unrestricted SpiceDB migration:

1. prevent `configs/resource/defaults.yaml` and `configs/spicedb/aisphere.schema.zed` from drifting;
2. install only schema elements that are missing, while refusing automatic rewrites or removals;
3. move bootstrap role expansion and control-plane admin resources out of Go constants and into the permission manifest.

## Current state

IAM currently starts three related flows:

- `NewResources` installs the SpiceDB schema and writes bootstrap administrator relationships;
- `defaults.ReconcileFile` upserts capabilities, resource types, permission catalogs, and role templates;
- generated access metadata enforces proto-declared API policies.

The resource catalog and SpiceDB schema describe the same resource permissions in two formats, but no repository gate compares them. Schema readiness checks only four top-level definitions, so a newly added relation or permission is ignored after the first installation. Bootstrap role aliases, expanded zone relations, and default IAM admin resources are hard-coded in `internal/data/data.go`.

## Considered approaches

### Always replace the active schema

Compare the entire active schema with the repository file and call `WriteSchema` whenever they differ. This makes convergence simple, but it can silently apply permission-expression changes or remove schema elements. Rejected because restart-time bootstrap must not become an unrestricted migration mechanism.

### Fail on every schema difference

Install only into an empty SpiceDB instance and reject all later differences. This is operationally safe, but additive permissions still require a manual publish step and does not satisfy the requested startup behavior. Rejected.

### Apply strict additive changes only

Parse the active and desired schemas into definitions, relations, and permissions. Existing declarations must be identical. Missing declarations are additive and may be published. Existing declarations that changed, or active declarations absent from the repository file, are conflicts and stop startup with a precise diagnostic. Selected.

SpiceDB still receives a complete schema through `WriteSchema`; the application-side classifier guarantees that the submitted repository schema is a strict additive superset of the active schema. SpiceDB validation remains the final type-safety gate.

## Architecture

### Permission manifest

`configs/resource/defaults.yaml` remains the single manifest for IAM initialization metadata. It will continue to contain capabilities, resource types, and role templates and gain a `bootstrap` section:

```yaml
bootstrap:
  default_role: zone_owner
  roles:
    zone_owner:
      aliases: [owner]
      zone_relations: [owner, admin, user_manager, group_manager, permission_admin]
      control_plane_admin: true
    zone_admin:
      aliases: [admin]
      zone_relations: [admin, user_manager, group_manager, permission_admin]
      control_plane_admin: true
  admin_resources:
    - {type: iam, id: organization}
    - {type: iam_authz, id: global}
```

All current role mappings and all nine current control-plane resources will be represented explicitly. Unknown roles will be rejected instead of becoming arbitrary SpiceDB relation names. Bootstrap subject identities remain deployment configuration because they are environment-specific, not permission-model metadata.

A neutral `internal/permissionmanifest` package will own manifest types, loading, validation, and schema/catalog consistency checks. Both startup data wiring and the defaults reconciler will consume this package, avoiding a data-to-business package dependency.

### Additive schema convergence

`BootstrapAuthzSchema` will always load and parse the desired `.zed` when default schema installation is enabled. It will then:

1. read and parse the active schema;
2. skip when the active and desired structural declarations are identical;
3. classify repository-only definitions, relations, and permissions as additions;
4. validate and publish the complete desired schema only when the active schema is a strict structural subset;
5. reject changed declarations and active-only declarations with an error listing the conflicting paths.

Whitespace and comments will not count as changes. Relation type expressions and permission expressions will be normalized for insignificant whitespace only; semantic expression changes remain explicit migration work.

### Consistency gate

The permission manifest validator will enforce:

- every manifest resource type maps to a SpiceDB definition;
- manifest relations and permissions exactly match that definition;
- every role-template relation exists on its resource type;
- every bootstrap zone relation exists on `zone`;
- bootstrap control-plane resources refer to known schema definitions and non-empty IDs;
- role aliases are unique and the default role resolves.

A `permission-manifest-check` command and Make target will run this validation without contacting SpiceDB. `make verify` and CI will include the target so drift cannot merge unnoticed.

### Startup data flow

The manifest is loaded once before authorization bootstrap and passed to both initialization paths:

1. construct database, identity, and SpiceDB providers;
2. load and validate the permission manifest against the desired `.zed`;
3. apply safe additive schema convergence;
4. resolve configured bootstrap subjects and expand relationships from manifest policy;
5. reconcile capabilities, resource types, and role templates using the same in-memory manifest;
6. start servers.

Any load, consistency, schema conflict, subject resolution, or relationship-write failure stops startup. Existing relationship writes and default reconciliation remain idempotent.

## Compatibility and deployment

The existing `control_plane.defaults.path` remains the manifest path, so local, test, Docker, and Kubernetes configurations retain one file setting. The `bootstrap_admins.subjects` configuration remains unchanged. The unused `bootstrap_admins.resources` override is removed from the supported contract so environment configuration cannot silently diverge from the permission manifest.

The Docker image already copies `configs/`, so no new volume is required. Deployment documentation will state that additive schema changes apply automatically while modifications and removals require the authorization-admin API or a controlled migration.

## Tests

Implementation will follow red-green-refactor cycles for:

- manifest parsing, alias resolution, and validation failures;
- schema parser normalization;
- identical-schema skip;
- empty-schema initial install;
- missing definition/relation/permission additive publish;
- changed relation or permission rejection;
- active-only declaration rejection;
- bootstrap relationship expansion from manifest data;
- end-to-end repository consistency for the committed manifest and `.zed` file;
- Make/CI target wiring.

The final verification set is targeted package tests, `go test ./...`, `make permission-manifest-check`, `make proto-check`, `make traceability-check STRICT=1`, `go build ./cmd/aisphere-iam`, and `git diff --check`.
