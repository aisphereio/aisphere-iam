// Package data authz_bootstrap.go — startup-time SpiceDB schema loader.
//
// NewResources owns the only schema-bootstrap call path. Startup reads the
// existing SpiceDB schema first and skips writes when the required IAM/zone
// definitions are already present. This prevents restart-time schema churn and
// avoids wrapping IdentityAdmin twice.
package data

import (
	"context"
	"os"
	"strings"

	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/logx"

	"github.com/aisphereio/aisphere-iam/internal/conf"
)

// IAMAuthzSchemaVersion is the fallback embedded schema version. Bump this when
// the fallback text below changes. The preferred production schema source is
// configs/spicedb/aisphere.schema.zed via security.authz.schema_path.
const IAMAuthzSchemaVersion = "1.0.2"

const IAMAuthzSchema = `definition user {}
definition service {}
definition service_account {}

definition zone {
  relation member: user | service | service_account | group#member
  relation owner: user | service | service_account | group#member
  relation admin: user | service | service_account | group#member
  relation user_viewer: user | service | service_account | group#member
  relation user_manager: user | service | service_account | group#member
  relation group_viewer: user | service | service_account | group#member
  relation group_manager: user | service | service_account | group#member
  relation permission_admin: user | service | service_account | group#member

  permission view_zone = member + owner + admin
  permission view_users = owner + admin + user_viewer + user_manager
  permission manage_users = owner + admin + user_manager
  permission view_groups = owner + admin + group_viewer + group_manager
  permission create_groups = owner + admin + group_manager
  permission manage_groups = owner + admin + group_manager
  permission view_permissions = owner + admin + permission_admin
  permission manage_permissions = owner + admin + permission_admin
}

definition group {
  relation zone: zone
  relation parent: group
  relation member: user | service | service_account | group#member
  relation owner: user | service | service_account | group#member
  relation manager: user | service | service_account | group#member
  relation viewer: user | service | service_account | group#member
  relation permission_admin: user | service | service_account | group#member

  permission view = member + viewer + manager + owner + zone->view_groups + parent->view
  permission manage = owner + manager + zone->manage_groups + parent->manage
  permission create_child_groups = owner + manager + zone->create_groups + parent->create_child_groups
  permission manage_members = owner + manager + zone->manage_users + parent->manage_members
  permission view_permissions = owner + permission_admin + zone->view_permissions + parent->view_permissions
  permission manage_permissions = owner + permission_admin + zone->manage_permissions + parent->manage_permissions
}

definition iam_authz {
  relation owner: user | service | service_account | group#member
  relation admin: user | service | service_account | group#member
  relation schema_admin: user | service | service_account | group#member
  relation auditor: user | service | service_account | group#member

  permission view_schema = owner + admin + schema_admin + auditor
  permission publish_schema = owner + admin + schema_admin
  permission view_relationships = owner + admin + schema_admin + auditor
  permission repair_relationships = owner + admin + schema_admin
  permission manage = owner + admin + schema_admin
}

definition iam {
  relation admin: user | group#member | service | service_account
  permission create = admin
  permission read = admin
  permission list = admin
  permission manage = admin
  permission create_project = admin
  permission upsert = admin
  permission bind = admin
  permission unbind = admin
  permission move = admin
  permission archive = admin
  permission delete = admin
  permission grant = admin
  permission revoke = admin
  permission explain = admin
}`

// BootstrapAuthzSchema is restart-safe: it writes only when the current schema is
// empty or missing the required IAM directory/authz definitions. It never wraps
// identity providers and therefore has no runtime mutation side effects.
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

	body := IAMAuthzSchema
	if path := strings.TrimSpace(cfg.SchemaPath); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		body = string(data)
	}
	if strings.TrimSpace(body) == "" {
		return authz.ErrBackendFailed("authz schema bootstrap source is empty", nil)
	}
	if err := resources.AuthzAdmin.ValidateSchema(ctx, authz.Schema{Text: body}); err != nil {
		return err
	}
	if err := resources.AuthzAdmin.WriteSchema(ctx, authz.Schema{Text: body}); err != nil {
		log.WithContext(ctx).Error("authz schema bootstrap failed", logx.Err(err))
		return err
	}
	log.WithContext(ctx).Info("authz schema bootstrapped", logx.Int("size", len(body)), logx.String("schema_version", IAMAuthzSchemaVersion))
	return nil
}

func schemaReady(schema string) bool {
	normalized := strings.ToLower(schema)
	return strings.Contains(normalized, "definition iam ") &&
		strings.Contains(normalized, "definition iam_authz ") &&
		strings.Contains(normalized, "definition zone ") &&
		strings.Contains(normalized, "definition group ")
}
