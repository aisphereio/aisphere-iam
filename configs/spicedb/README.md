# SpiceDB schema workflow

`configs/spicedb/aisphere.schema.zed` is the single source of truth for the IAM SpiceDB schema.

Do not copy the schema into Go code. Do not maintain a second copy inside `deploy/configmap.yaml`.

## Runtime bootstrap

IAM loads the schema from `security.authz.schema_path` when `security.authz.install_default_schema` is enabled.

Recommended local configuration:

```yaml
security:
  authz:
    enabled: true
    provider: spicedb
    install_default_schema: true
    schema_path: configs/spicedb/aisphere.schema.zed
```

Recommended Kubernetes configuration:

```yaml
security:
  authz:
    enabled: true
    provider: spicedb
    install_default_schema: true
    schema_path: /app/configs/spicedb/aisphere.schema.zed
```

Startup is idempotent and additive-only. IAM parses the active and desired schemas by definition, relation, and permission. Identical declarations are skipped; missing declarations are validated and published. Changed declarations or active declarations absent from the repository schema stop startup and require an explicit opt-in via `allow_permission_migrations` (expression changes) or `allow_schema_deletions` (removed declarations). The two flags are independent: opening one never permits the other class of conflict.

`configs/resource/defaults.yaml` is the initialization manifest for resource catalogs, role templates, bootstrap role expansion, and control-plane admin resources. Run `make permission-manifest-check` to verify that its relation and permission lists match this executable `.zed` schema.

## Kubernetes deployment

`make deploy-apply` creates or updates the schema ConfigMap from the source file:

```bash
make deploy-apply
```

The equivalent manual command is:

```bash
kubectl apply -f deploy/namespace.yaml
kubectl create configmap aisphere-iam-spicedb-schema \
  -n aisphere \
  --from-file=aisphere.schema.zed=configs/spicedb/aisphere.schema.zed \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/deployment.yaml
```

The IAM deployment mounts that ConfigMap at:

```text
/app/configs/spicedb/aisphere.schema.zed
```

## Production change policy

SpiceDB `WriteSchema` replaces the active schema. A schema edit can break existing relationships if a relation, permission, or object definition is renamed or removed.

Use this rule of thumb:

- Additive changes are usually safe: new definitions, new relations, new permissions.
- Destructive changes require a migration plan: rename/remove definitions, rename/remove relations, or change allowed subject types.
- Strict additions are applied by IAM startup after manifest and SpiceDB validation.
- Changed declarations require `security.authz.allow_permission_migrations: true`; removed declarations require `security.authz.allow_schema_deletions: true`. Both default to `false`, and startup refuses the corresponding conflicts when the gate is closed. Revert each flag to `false` once the migration is applied.

## Future improvements

Further production hardening can add schema version metadata, relationship compatibility reports, and dedicated jobs for destructive migrations.
