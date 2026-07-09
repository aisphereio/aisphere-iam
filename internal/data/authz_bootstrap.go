// Package data authz_bootstrap.go — startup-time SpiceDB schema loader.
//
// NewResources owns the only schema-bootstrap call path. Startup reads the
// existing SpiceDB schema first and skips writes when the required IAM/zone
// definitions are already present. This prevents restart-time schema churn and
// avoids wrapping IdentityAdmin twice.
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
)

// BootstrapAuthzSchema is restart-safe: it writes only when the current schema is
// empty or missing the required IAM directory/authz definitions. It never wraps
// identity providers and therefore has no runtime mutation side effects.
//
// When security.authz.install_default_schema is enabled, security.authz.schema_path
// is required. The schema file is validated before WriteSchema. WriteSchema
// replaces the active SpiceDB schema, so incompatible schema evolution must still
// be handled through an explicit migration/review process.
func BootstrapAuthzSchema(ctx context.Context, cfg conf.AuthzConfig, resources *Resources, log logx.Logger) error {
	if resources == nil || resources.AuthzAdmin == nil {
		if log != nil {
			log.WithContext(ctx).Info("authz schema bootstrap skipped: authz not configured")
		}
		return nil
	}
	if log == nil {
		log = logx.Noop()
	}
	log = log.Named("authz.bootstrap")

	schema, err := resources.AuthzAdmin.ReadSchema(ctx)
	if err != nil {
		log.WithContext(ctx).Warn("read schema failed; will attempt bootstrap", logx.Err(err))
	} else if schemaReady(schema.Text) {
		log.WithContext(ctx).Info("authz schema already installed; skipping bootstrap", logx.Int("size", len(schema.Text)))
		return nil
	}

	path := strings.TrimSpace(cfg.SchemaPath)
	if path == "" {
		return authz.ErrBackendFailed("security.authz.schema_path is required when install_default_schema is enabled", nil)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return authz.ErrBackendFailed("read authz schema file failed: "+path, err)
	}
	body := string(data)
	if strings.TrimSpace(body) == "" {
		return authz.ErrBackendFailed("authz schema file is empty: "+path, nil)
	}
	if err := resources.AuthzAdmin.ValidateSchema(ctx, authz.Schema{Text: body}); err != nil {
		return err
	}
	if err := resources.AuthzAdmin.WriteSchema(ctx, authz.Schema{Text: body}); err != nil {
		log.WithContext(ctx).Error("authz schema bootstrap failed", logx.Err(err), logx.String("schema_path", path))
		return err
	}
	log.WithContext(ctx).Info("authz schema bootstrapped", logx.Int("size", len(body)), logx.String("schema_path", path))
	return nil
}

func schemaReady(schema string) bool {
	normalized := strings.ToLower(schema)
	return strings.Contains(normalized, "definition iam ") &&
		strings.Contains(normalized, "definition iam_authz ") &&
		strings.Contains(normalized, "definition zone ") &&
		strings.Contains(normalized, "definition group ")
}
