-- +goose Up
-- Add fine-grained custom role support: role template permissions, audit
-- trail, version column for optimistic concurrency, and grant → role template
-- link. Introduced by ffd903e (fine-grained custom IAM roles).

-- 1. Role template version column (optimistic concurrency).
ALTER TABLE iam_role_templates
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;

-- 2. Role template permissions — many-to-many between role templates and
--    resource-type permissions they grant.
CREATE TABLE IF NOT EXISTS iam_role_template_permissions (
    id               TEXT PRIMARY KEY,
    role_template_id TEXT NOT NULL,
    permission       TEXT NOT NULL,
    sort_order       INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT fk_iam_role_template_permissions_template
        FOREIGN KEY (role_template_id)
        REFERENCES iam_role_templates (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_iam_role_permission
    ON iam_role_template_permissions (role_template_id, permission);

CREATE INDEX IF NOT EXISTS idx_iam_role_template_permissions_template
    ON iam_role_template_permissions (role_template_id);

-- 3. Role template audit — append-only history of create/update/disable/enable.
CREATE TABLE IF NOT EXISTS iam_role_template_audits (
    id               TEXT PRIMARY KEY,
    role_template_id TEXT NOT NULL,
    version          BIGINT NOT NULL,
    action           TEXT NOT NULL,
    actor_type       TEXT,
    actor_id         TEXT,
    before_json      JSONB NOT NULL DEFAULT '{}',
    after_json       JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_iam_role_template_audits_template
    ON iam_role_template_audits (role_template_id);

CREATE INDEX IF NOT EXISTS idx_iam_role_template_audits_action
    ON iam_role_template_audits (action);

-- 4. Grant → role template link (nullable; legacy grants keep role_key only).
ALTER TABLE iam_grants
    ADD COLUMN IF NOT EXISTS role_template_id TEXT;

CREATE INDEX IF NOT EXISTS idx_iam_grant_role_template
    ON iam_grants (role_template_id);

-- +goose Down
DROP INDEX IF EXISTS idx_iam_grant_role_template;
ALTER TABLE iam_grants DROP COLUMN IF EXISTS role_template_id;

DROP TABLE IF EXISTS iam_role_template_audits;
DROP TABLE IF EXISTS iam_role_template_permissions;

ALTER TABLE iam_role_templates DROP COLUMN IF EXISTS version;
