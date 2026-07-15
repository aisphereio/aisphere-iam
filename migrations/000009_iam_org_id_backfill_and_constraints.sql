-- +goose Up
-- Phase 1: Backfill org_id for existing rows.
-- iam_grants: derive org_id from the resource's org_id (via iam_resources) or project's org_id.
UPDATE iam_grants g
  SET org_id = COALESCE(
    (SELECT r.org_id FROM iam_resources r WHERE r.type = g.resource_type AND r.id = g.resource_id),
    (SELECT p.org_id FROM iam_projects p WHERE p.id = g.resource_id),
    ''
  )
WHERE g.org_id = '';

-- iam_grant_audits: derive org_id from the corresponding grant.
UPDATE iam_grant_audits a
  SET org_id = COALESCE(
    (SELECT g.org_id FROM iam_grants g WHERE g.id = a.grant_id),
    ''
  )
WHERE a.org_id = '';

-- iam_resource_bindings: derive org_id from the source resource's org_id.
UPDATE iam_resource_bindings b
  SET org_id = COALESCE(
    (SELECT r.org_id FROM iam_resources r WHERE r.type = b.source_type AND r.id = b.source_id),
    ''
  )
WHERE b.org_id = '';

-- iam_external_resource_bindings: derive org_id from the resource's org_id.
UPDATE iam_external_resource_bindings b
  SET org_id = COALESCE(
    (SELECT r.org_id FROM iam_resources r WHERE r.type = b.resource_type AND r.id = b.resource_id),
    ''
  )
WHERE b.org_id = '';

-- iam_role_templates: built-in templates stay global (org_id = ''), custom ones
-- need manual assignment. No automatic backfill for custom templates.

-- Phase 2: Add NOT NULL constraints on tables that must always have an org.
-- iam_grants, iam_grant_audits, iam_resource_bindings, iam_external_resource_bindings
-- must have org_id. iam_outbox_events and iam_role_templates remain nullable.

ALTER TABLE iam_grants ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE iam_grant_audits ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE iam_resource_bindings ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE iam_external_resource_bindings ALTER COLUMN org_id SET NOT NULL;

-- Phase 3: Add composite unique keys that include org_id for org-scoped uniqueness.
-- iam_grants: prevent duplicate active grants within the same org.
-- A grant is uniquely identified by (org_id, resource_type, resource_id, role_key, subject_type, subject_id, subject_relation).
-- We use a partial unique index to only enforce uniqueness on active (non-revoked) grants.
CREATE UNIQUE INDEX IF NOT EXISTS uq_iam_grants_active
  ON iam_grants(org_id, resource_type, resource_id, role_key, subject_type, subject_id, COALESCE(subject_relation, ''))
  WHERE revoked_at IS NULL;

-- iam_resource_bindings: include org_id in the unique constraint.
-- Drop the old global unique constraint and replace with org-scoped one.
ALTER TABLE iam_resource_bindings DROP CONSTRAINT IF EXISTS uq_iam_resource_binding;
CREATE UNIQUE INDEX IF NOT EXISTS uq_iam_resource_binding_org
  ON iam_resource_bindings(org_id, source_type, source_id, relation, target_type, target_id);

-- iam_external_resource_bindings: include org_id in the unique constraint.
ALTER TABLE iam_external_resource_bindings DROP CONSTRAINT IF EXISTS uq_iam_external_resource_binding;
CREATE UNIQUE INDEX IF NOT EXISTS uq_iam_external_resource_binding_org
  ON iam_external_resource_bindings(org_id, provider, external_type, external_id);

-- iam_role_templates: custom templates (non-built-in) are unique per org per resource_type per role_key.
-- Built-in templates (org_id = '') remain global.
CREATE UNIQUE INDEX IF NOT EXISTS uq_iam_role_template_org_resource_role
  ON iam_role_templates(org_id, resource_type, role_key)
  WHERE org_id != '';

-- +goose Down
-- Phase 3 down: drop composite unique indexes.
DROP INDEX IF EXISTS uq_iam_grants_active;
DROP INDEX IF EXISTS uq_iam_resource_binding_org;
DROP INDEX IF EXISTS uq_iam_external_resource_binding_org;
DROP INDEX IF EXISTS uq_iam_role_template_org_resource_role;

-- Restore original unique constraints (without org_id).
CREATE UNIQUE INDEX IF NOT EXISTS uq_iam_resource_binding ON iam_resource_bindings(source_type, source_id, relation, target_type, target_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_iam_external_resource_binding ON iam_external_resource_bindings(provider, external_type, external_id);

-- Phase 2: drop NOT NULL constraints.
ALTER TABLE iam_grants ALTER COLUMN org_id DROP NOT NULL;
ALTER TABLE iam_grant_audits ALTER COLUMN org_id DROP NOT NULL;
ALTER TABLE iam_resource_bindings ALTER COLUMN org_id DROP NOT NULL;
ALTER TABLE iam_external_resource_bindings ALTER COLUMN org_id DROP NOT NULL;

-- Phase 1: no rollback for data backfill (data is already migrated).