// Package data authz_bootstrap.go — startup-time SpiceDB schema loader.
//
// On IAM startup, after Resources are constructed, BootstrapAuthzSchema
// checks whether SpiceDB already has a schema. If not (or if the schema
// is empty), it writes a default schema that covers all resource types
// the IAM control plane uses.
//
// This is idempotent: WriteSchema on SpiceDB replaces the schema in
// place, so re-running on an already-initialized SpiceDB is safe (but
// will invalidate any tuples that reference relations not present in
// the new schema — operators should review the schema text before
// deploying a new IAM version).
//
// The default schema is sourced from kernel/authz/spicedb.DefaultSchema
// extended with IAM-specific resource types. Keeping the kernel default
// ensures platform / organization / group / application / project /
// resource types stay in sync with the kernel IAM projection layer.

package data

import (
	"context"
	"strings"

	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/logx"
)

// IAMAuthzSchemaVersion is the schema version. Bump this when the schema
// text below changes.
const IAMAuthzSchemaVersion = "1.0.1"

// IAMAuthzSchema is the default SpiceDB schema for aisphere-iam. It
// extends kernel/authz/spicedb.DefaultSchema with the iam control-plane
// resource type and its relations/permissions.
//
// The "iam" type represents control-plane admin resources (organization,
// capability, resource_type, user, group, relationship, etc.) that are
// bootstrapped with admin relationships. The listed permissions must stay
// aligned with aisphere.access.v1.policy actions in IAM proto contracts.
const IAMAuthzSchema = `definition user {}
definition service {}

definition platform {
  relation super_admin: user | service
  permission admin = super_admin
}

definition iam {
  relation admin: user | service

  permission create = admin
  permission read = admin
  permission list = admin
  permission update = admin
  permission disable = admin
  permission delete = admin
  permission manage = admin
  permission assign = admin
  permission remove = admin
  permission bind = admin
  permission unbind = admin
  permission move = admin
  permission archive = admin
  permission create_project = admin
  permission grant = admin
  permission revoke = admin
  permission explain = admin
  permission write = admin
  permission lookup = admin
  permission check = admin
}

definition organization {
  relation platform: platform
  relation owner: user | service
  relation admin: user | service | group#member
  relation member: user | service | group#member

  permission manage = owner + admin + platform->admin
  permission read = owner + admin + member + platform->admin
}

definition group {
  relation org: organization
  relation parent: group
  relation member: user | service | group#member

  permission read = member + parent->read + org->read
}

definition application {
  relation org: organization
  relation owner: user | service
  relation admin: user | service | group#member
  relation member: user | service | group#member

  permission manage = owner + admin + org->manage
  permission read = owner + admin + member + org->read
}

definition project {
  relation org: organization
  relation owner: user | service
  relation editor: user | service | group#member
  relation viewer: user | service | group#member

  permission read = viewer + editor + owner + org->read
  permission edit = editor + owner + org->manage
  permission delete = owner + org->manage
}

definition resource {
  relation project: project
  relation owner: user | service
  relation editor: user | service | group#member
  relation viewer: user | service | group#member

  permission read = viewer + editor + owner + project->read
  permission edit = editor + owner + project->edit
  permission delete = owner + project->delete
}`

// BootstrapAuthzSchema writes the default IAM authz schema to SpiceDB
// when the schema is empty or missing. Called from main.go after
// Resources are constructed.
//
// Behavior:
//   - Binds identity group operations to AuthZ relationship projection when
//     AuthzAdmin is configured.
//   - If AuthzAdmin is nil (authz disabled), returns nil immediately.
//   - If ReadSchema returns an error other than "schema not found",
//     returns the error so main.go can surface it.
//   - If ReadSchema returns a non-empty schema text that already contains
//     IAM definitions (definition iam), returns nil.
//   - If ReadSchema returns a non-empty schema text that only contains the
//     Kernel base definitions, writes IAMAuthzSchema to add the iam type.
//   - If ReadSchema returns empty schema text, writes IAMAuthzSchema.
//
// The function is safe to call on every startup — it only writes when
// the schema is empty, so re-running on an initialized SpiceDB is a
// no-op.
func BootstrapAuthzSchema(ctx context.Context, resources *Resources, log logx.Logger) error {
	if resources != nil {
		resources.Identity = BindIdentityAuthZ(resources.Identity, resources.AuthzAdmin)
	}
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
		log.WithContext(ctx).Warn("read schema failed; will attempt to write default",
			logx.Err(err),
		)
	} else if schema.Text != "" {
		if hasIAMAuthzDefinitions(schema.Text) {
			log.WithContext(ctx).Info("authz schema already installed; skipping bootstrap",
				logx.Int("size", len(schema.Text)),
			)
			return nil
		}
		log.WithContext(ctx).Warn("authz schema missing IAM definitions; applying IAM schema",
			logx.Int("current_size", len(schema.Text)),
			logx.String("schema_version", IAMAuthzSchemaVersion),
		)
	}

	if err := resources.AuthzAdmin.WriteSchema(ctx, authz.Schema{Text: IAMAuthzSchema}); err != nil {
		log.WithContext(ctx).Error("authz schema bootstrap failed",
			logx.Err(err),
		)
		return err
	}
	log.WithContext(ctx).Info("authz schema bootstrapped",
		logx.Int("size", len(IAMAuthzSchema)),
	)
	return nil
}

func hasIAMAuthzDefinitions(schema string) bool {
	normalized := strings.ToLower(schema)
	return strings.Contains(normalized, "definition iam ")
}
