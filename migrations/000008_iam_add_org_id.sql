-- +goose Up
-- Add org_id to iam_grants for org-scoped grant isolation.
ALTER TABLE iam_grants ADD COLUMN IF NOT EXISTS org_id text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_iam_grants_org_id ON iam_grants(org_id);

-- Add org_id to iam_grant_audits for org-scoped audit trail.
ALTER TABLE iam_grant_audits ADD COLUMN IF NOT EXISTS org_id text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_iam_grant_audits_org_id ON iam_grant_audits(org_id);

-- Add org_id to iam_outbox_events (nullable: system events may not have an org context).
ALTER TABLE iam_outbox_events ADD COLUMN IF NOT EXISTS org_id text DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_iam_outbox_events_org_id ON iam_outbox_events(org_id);

-- Add org_id to iam_resource_bindings for org-scoped binding isolation.
ALTER TABLE iam_resource_bindings ADD COLUMN IF NOT EXISTS org_id text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_iam_resource_bindings_org_id ON iam_resource_bindings(org_id);

-- Add org_id to iam_external_resource_bindings for org-scoped external binding isolation.
ALTER TABLE iam_external_resource_bindings ADD COLUMN IF NOT EXISTS org_id text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_iam_external_bindings_org_id ON iam_external_resource_bindings(org_id);

-- Add org_id to iam_role_templates (nullable: built-in templates are global, custom ones are org-scoped).
ALTER TABLE iam_role_templates ADD COLUMN IF NOT EXISTS org_id text DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_iam_role_templates_org_id ON iam_role_templates(org_id);

-- +goose Down
DROP INDEX IF EXISTS idx_iam_grants_org_id;
ALTER TABLE iam_grants DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_iam_grant_audits_org_id;
ALTER TABLE iam_grant_audits DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_iam_outbox_events_org_id;
ALTER TABLE iam_outbox_events DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_iam_resource_bindings_org_id;
ALTER TABLE iam_resource_bindings DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_iam_external_bindings_org_id;
ALTER TABLE iam_external_resource_bindings DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_iam_role_templates_org_id;
ALTER TABLE iam_role_templates DROP COLUMN IF EXISTS org_id;