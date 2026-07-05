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

// IAMAuthzSchema is the default SpiceDB schema for aisphere-iam. The iam
// permissions must stay aligned with aisphere.access.v1.policy actions in
// IAM proto contracts.
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

var requiredIAMAuthzPermissions = []string{
	"create", "read", "list", "update", "disable", "delete", "manage",
	"assign", "remove", "bind", "unbind", "move", "archive", "create_project",
	"grant", "revoke", "explain", "write", "lookup", "check",
}

// BootstrapAuthzSchema writes the default IAM authz schema to SpiceDB when the
// schema is missing or lacks the IAM permissions required by generated policy.
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
		log.WithContext(ctx).Warn("read schema failed; will attempt to write default", logx.Err(err))
	} else if schema.Text != "" {
		if hasIAMAuthzDefinitions(schema.Text) && hasRequiredIAMAuthzPermissions(schema.Text) {
			log.WithContext(ctx).Info("authz schema already installed; skipping bootstrap",
				logx.Int("size", len(schema.Text)),
				logx.String("schema_version", IAMAuthzSchemaVersion),
			)
			return nil
		}
		log.WithContext(ctx).Warn("authz schema missing IAM definitions or permissions; applying IAM schema",
			logx.Int("current_size", len(schema.Text)),
			logx.String("schema_version", IAMAuthzSchemaVersion),
		)
	}

	if err := resources.AuthzAdmin.WriteSchema(ctx, authz.Schema{Text: IAMAuthzSchema}); err != nil {
		log.WithContext(ctx).Error("authz schema bootstrap failed", logx.Err(err))
		return err
	}
	log.WithContext(ctx).Info("authz schema bootstrapped",
		logx.Int("size", len(IAMAuthzSchema)),
		logx.String("schema_version", IAMAuthzSchemaVersion),
	)
	return nil
}

func hasIAMAuthzDefinitions(schema string) bool {
	normalized := strings.ToLower(schema)
	return strings.Contains(normalized, "definition iam ")
}

func hasRequiredIAMAuthzPermissions(schema string) bool {
	normalized := strings.ToLower(schema)
	for _, permission := range requiredIAMAuthzPermissions {
		if !strings.Contains(normalized, "permission "+permission+" =") {
			return false
		}
	}
	return true
}
