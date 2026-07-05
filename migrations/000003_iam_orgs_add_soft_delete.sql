-- +goose Up
ALTER TABLE iam_organizations
  ADD COLUMN IF NOT EXISTS created_by text,
  ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_iam_organizations_deleted_at ON iam_organizations(deleted_at);

-- +goose Down
DROP INDEX IF EXISTS idx_iam_organizations_deleted_at;
ALTER TABLE iam_organizations
  DROP COLUMN IF EXISTS deleted_at,
  DROP COLUMN IF EXISTS created_by;