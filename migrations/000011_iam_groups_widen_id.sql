-- +goose Up
-- Stable group IDs use "grp_" followed by 32 hexadecimal characters, so the
-- original VARCHAR(32) column cannot store them. Keep headroom for future
-- provider-neutral stable ID formats.
ALTER TABLE iam_groups ALTER COLUMN id TYPE VARCHAR(64);

-- +goose Down
-- Intentionally no-op: shrinking the primary key could truncate stable IDs
-- that are valid under the current contract.
