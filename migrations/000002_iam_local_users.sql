-- +goose Up
CREATE TABLE IF NOT EXISTS iam_local_users (
  username text PRIMARY KEY,
  subject_id text,
  subject_type text NOT NULL DEFAULT 'human',
  display_name text,
  email text,
  organization text,
  roles_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  permissions_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  namespaces_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  password_hash text,
  disabled boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_iam_local_users_subject ON iam_local_users(subject_id, subject_type);
CREATE INDEX IF NOT EXISTS idx_iam_local_users_disabled ON iam_local_users(disabled);

-- +goose Down
DROP TABLE IF EXISTS iam_local_users;