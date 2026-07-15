-- +goose Up
-- Add metadata_json column to iam_projects.
--
-- ProjectModel (internal/data/resource_models.go) declares a MetadataJSON
-- field mapped to the metadata_json column, but migration 000001 omitted it
-- from the CREATE TABLE statement. GORM therefore generates INSERTs that
-- reference a non-existent column, causing PostgreSQL SQLSTATE 42P01
-- ("column ... of relation ... does not exist") on every CreateProject call.

ALTER TABLE iam_projects
    ADD COLUMN IF NOT EXISTS metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE iam_projects
    DROP COLUMN IF EXISTS metadata_json;
