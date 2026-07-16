-- +goose Up
-- iam_groups stores the IAM-managed Group mapping table.
--
-- Groups are created in Casdoor (the identity provider) with:
--   Name        = IAM-generated stable ID (e.g. "grp_01AR...")
--   DisplayName = user-visible display name (e.g. "E2E 顶级组织")
--
-- This table maps the IAM stable ID to the Casdoor group name and stores
-- the machine-readable name (slug) that is NOT persisted in Casdoor.
CREATE TABLE iam_groups (
    id              VARCHAR(32)  PRIMARY KEY,
    org_id          VARCHAR(128) NOT NULL,
    parent_id       VARCHAR(64),
    name            VARCHAR(128) NOT NULL,
    display_name    VARCHAR(128) NOT NULL DEFAULT '',
    casdoor_name    VARCHAR(128) NOT NULL,
    type            VARCHAR(32)  NOT NULL DEFAULT 'Physical',
    status          VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at      TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP    NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_iam_groups_org_name ON iam_groups (org_id, name);
CREATE INDEX IF NOT EXISTS idx_iam_groups_org_id ON iam_groups (org_id);
CREATE INDEX IF NOT EXISTS idx_iam_groups_parent_id ON iam_groups (parent_id);
CREATE INDEX IF NOT EXISTS idx_iam_groups_casdoor_name ON iam_groups (casdoor_name);
CREATE INDEX IF NOT EXISTS idx_iam_groups_status ON iam_groups (status);

-- +goose Down
DROP TABLE IF EXISTS iam_groups;