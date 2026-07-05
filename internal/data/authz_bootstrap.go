package data

import (
	"context"
	"strings"

	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/logx"
)

const IAMAuthzSchemaVersion = "1.0.2"

// IAMAuthzSchema is the default SpiceDB schema for aisphere-iam. The
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
  permission upsert = admin
  permission refresh = admin
  permission verify = admin
}

definition organization {
  relation platform: platform
  relation owner: user | service
  relation admin: user | service | group#member
  relation member: user | service | group#member

  permission manage = owner + admin + platform->admin
  permission read = owner + admin + member + platform->admin
  permission list = owner + admin + member + platform->admin
  permission create_project = owner + admin + platform->admin
}

definition group {
  relation org: organization
  relation parent: group
  relation member: user | service | group#member

  permission read = member + parent->read + org->read
  permission list = org->read
  permission manage = org->manage
  permission assign = org->manage
  permission remove = org->manage
  permission delete = org->manage
}

definition application {
  relation org: organization
  relation owner: user | service
  relation admin: user | service | group#member
  relation member: user | service | group#member

  permission manage = owner + admin + org->manage
  permission read = owner + admin + member + org->read
  permission list = org->read
}

definition project {
  relation org: organization
  relation owner: user | service
  relation editor: user | service | group#member
  relation viewer: user | service | group#member

  permission read = viewer + editor + owner + org->read
  permission list = org->read
  permission edit = editor + owner + org->manage
  permission manage = editor + owner + org->manage
  permission archive = owner + org->manage
  permission delete = owner + org->manage
}

definition resource {
  relation project: project
  relation owner: user | service
  relation editor: user | service | group#member
  relation viewer: user | service | group#member

  permission read = viewer + editor + owner + project->read
  permission list = project->read
  permission edit = editor + owner + project->edit
  permission manage = editor + owner + project->manage
  permission move = owner + project->manage
  permission archive = owner + project->manage
  permission delete = owner + project->delete
}`

var requiredSchemaPermissions = map[string][]string{
	"iam": {
		"create", "read", "list", "update", "disable", "delete", "manage",
		"assign", "remove", "bind", "unbind", "move", "archive", "create_project",
		"grant", "revoke", "explain", "write", "lookup", "check", "upsert",
		"refresh", "verify",
	},
	"organization": {"read", "list", "manage", "create_project"},
	"group":        {"read", "list", "manage", "assign", "remove", "delete"},
	"application":  {"read", "list", "manage"},
	"project":      {"read", "list", "edit", "manage", "archive", "delete"},
	"resource":     {"read", "list", "edit", "manage", "move", "archive", "delete"},
}

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
		if hasRequiredSchemaPermissions(schema.Text) {
			log.WithContext(ctx).Info("authz schema already installed; skipping bootstrap",
				logx.Int("size", len(schema.Text)),
				logx.String("schema_version", IAMAuthzSchemaVersion),
			)
			return nil
		}
		log.WithContext(ctx).Warn("authz schema missing definitions or permissions; applying IAM schema",
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

func hasRequiredSchemaPermissions(schema string) bool {
	normalized := strings.ToLower(schema)
	for definition, permissions := range requiredSchemaPermissions {
		block := schemaDefinitionBlock(normalized, definition)
		if block == "" {
			return false
		}
		for _, permission := range permissions {
			if !strings.Contains(block, "permission "+permission+" =") {
				return false
			}
		}
	}
	return true
}

func schemaDefinitionBlock(schema string, definition string) string {
	start := strings.Index(schema, "definition "+strings.ToLower(definition)+" ")
	if start < 0 {
		return ""
	}
	rest := schema[start+len("definition "+definition+" "):]
	next := strings.Index(rest, "\ndefinition ")
	if next < 0 {
		return schema[start:]
	}
	return schema[start : start+len("definition "+definition+" ")+next]
}
