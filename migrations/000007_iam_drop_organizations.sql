-- +goose Up
-- The OrganizationModel / iam_organizations table was a control-plane mirror
-- of Casdoor organizations (zones). Zone identity now lives solely in Casdoor
-- (and the SpiceDB `zone` object); iam_projects and iam_resources keep org_id
-- as a plain text column referencing the zone. Drop the foreign keys first so
-- the table can be removed without breaking references.

ALTER TABLE iam_projects DROP CONSTRAINT IF EXISTS iam_projects_org_id_fkey;
ALTER TABLE iam_resources DROP CONSTRAINT IF EXISTS iam_resources_org_id_fkey;
DROP TABLE IF EXISTS iam_organizations;

-- +goose Down
-- Recreating iam_organizations is intentionally a no-op: the control-plane no
-- longer maintains a local organizations table. Restoring the pre-000007
-- schema requires re-running 000001's CREATE TABLE manually.
