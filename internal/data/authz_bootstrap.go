// Package data authz_bootstrap.go — startup-time SpiceDB schema loader.
//
// NewResources owns the only schema-bootstrap call path. Startup compares the
// active and desired schemas and publishes only strict structural additions.
//
// SpiceDB schema is intentionally file-backed. Do not hard-code .zed schema text
// in Go code: schema changes are operational/configuration changes and should be
// reviewed, mounted, validated, and applied from configs/spicedb/*.zed or a
// Kubernetes ConfigMap.
package data

import (
	"context"
	"os"
	"strings"

	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/logx"

	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/permissionmanifest"
)

// BootstrapAuthzSchema is restart-safe: it skips identical schemas, publishes
// strict additions, and rejects changed or removed declarations.
//
// When security.authz.install_default_schema is enabled, security.authz.schema_path
// is required. The schema file is validated before WriteSchema. WriteSchema
// replaces the active SpiceDB schema, so incompatible schema evolution must still
// be handled through an explicit migration/review process.
func BootstrapAuthzSchema(ctx context.Context, cfg conf.AuthzConfig, manager authz.SchemaManager, log logx.Logger) error {
	if manager == nil {
		if log != nil {
			log.WithContext(ctx).Info("authz schema bootstrap skipped: authz not configured")
		}
		return nil
	}
	if log == nil {
		log = logx.Noop()
	}
	log = log.Named("authz.bootstrap")

	path := strings.TrimSpace(cfg.SchemaPath)
	if path == "" {
		return authz.ErrBackendFailed("security.authz.schema_path is required when install_default_schema is enabled", nil)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return authz.ErrBackendFailed("read authz schema file failed: "+path, err)
	}
	desiredText := string(data)
	if strings.TrimSpace(desiredText) == "" {
		return authz.ErrBackendFailed("authz schema file is empty: "+path, nil)
	}
	desired, err := permissionmanifest.ParseSchema(desiredText)
	if err != nil {
		return authz.ErrBackendFailed("parse desired authz schema failed", err)
	}
	activeSchema, err := manager.ReadSchema(ctx)
	if err != nil {
		return authz.ErrBackendFailed("read active authz schema failed", err)
	}
	active, err := permissionmanifest.ParseSchema(activeSchema.Text)
	if err != nil {
		return authz.ErrBackendFailed("parse active authz schema failed", err)
	}
	diff := permissionmanifest.CompareSchemas(active, desired)
	if len(diff.Conflicts) > 0 {
		if !cfg.AllowPermissionMigrations || !onlyChangedSchemaExpressions(diff.Conflicts) {
			return authz.ErrBackendFailed("authz schema drift requires explicit migration: "+strings.Join(diff.Conflicts, "; "), nil)
		}
	}
	if diff.Identical() {
		log.WithContext(ctx).Info("authz schema already installed; skipping bootstrap", logx.Int("size", len(activeSchema.Text)))
		return nil
	}

	desiredSchema := authz.Schema{Text: desiredText}
	if err := manager.ValidateSchema(ctx, desiredSchema); err != nil {
		return err
	}
	if err := manager.WriteSchema(ctx, desiredSchema); err != nil {
		log.WithContext(ctx).Error("authz schema bootstrap failed", logx.Err(err), logx.String("schema_path", path))
		return err
	}
	log.WithContext(ctx).Info("authz schema applied", logx.Int("additions", len(diff.Additions)), logx.Int("permission_migrations", len(diff.Conflicts)), logx.String("schema_path", path))
	return nil
}

func onlyChangedSchemaExpressions(conflicts []string) bool {
	if len(conflicts) == 0 {
		return false
	}
	for _, conflict := range conflicts {
		if !strings.HasSuffix(strings.TrimSpace(conflict), " changed") {
			return false
		}
	}
	return true
}
