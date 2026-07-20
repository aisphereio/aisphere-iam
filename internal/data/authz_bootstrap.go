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
		if err := authorizeSchemaConflicts(diff.Conflicts, cfg); err != nil {
			return err
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
	log.WithContext(ctx).Info("authz schema applied", logx.Int("additions", len(diff.Additions)), logx.Int("permission_migrations", countChangedConflicts(diff.Conflicts)), logx.Int("schema_deletions", countDeletedConflicts(diff.Conflicts)), logx.String("schema_path", path))
	return nil
}

// authorizeSchemaConflicts enforces the additive-only policy with two opt-in
// escape hatches. Expression changes ("X changed") require AllowPermissionMigrations;
// removed declarations ("X exists only in active schema") require AllowSchemaDeletions.
// Each gate is independent: opening one never permits the other class of conflict.
func authorizeSchemaConflicts(conflicts []string, cfg conf.AuthzConfig) error {
	var blocked []string
	for _, conflict := range conflicts {
		trimmed := strings.TrimSpace(conflict)
		switch {
		case strings.HasSuffix(trimmed, " changed"):
			if !cfg.AllowPermissionMigrations {
				blocked = append(blocked, conflict)
			}
		case strings.HasSuffix(trimmed, " exists only in active schema"):
			if !cfg.AllowSchemaDeletions {
				blocked = append(blocked, conflict)
			}
		default:
			blocked = append(blocked, conflict)
		}
	}
	if len(blocked) > 0 {
		return authz.ErrBackendFailed("authz schema drift requires explicit migration: "+strings.Join(blocked, "; "), nil)
	}
	return nil
}

func countChangedConflicts(conflicts []string) int {
	n := 0
	for _, conflict := range conflicts {
		if strings.HasSuffix(strings.TrimSpace(conflict), " changed") {
			n++
		}
	}
	return n
}

func countDeletedConflicts(conflicts []string) int {
	n := 0
	for _, conflict := range conflicts {
		if strings.HasSuffix(strings.TrimSpace(conflict), " exists only in active schema") {
			n++
		}
	}
	return n
}
