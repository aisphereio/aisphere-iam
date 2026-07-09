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

Startup is idempotent: IAM first reads the active SpiceDB schema and skips bootstrap when the required IAM directory definitions already exist.

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
- For schema upgrades after initial bootstrap, prefer the IAM authorization admin publish API or a controlled operator job instead of automatic restart-time overwrite.

## Future improvements

Planned production hardening:

- schema version metadata
- schema diff / preflight check
- relationship compatibility checks before publish
- separate schema migration jobs for destructive changes
