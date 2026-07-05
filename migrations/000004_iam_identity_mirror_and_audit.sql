-- +goose Up
CREATE TABLE IF NOT EXISTS iam_identity_users (
    id              TEXT PRIMARY KEY,
    provider        TEXT NOT NULL,
    provider_owner  TEXT NOT NULL,
    provider_name   TEXT NOT NULL,
    username        TEXT NOT NULL,
    display_name    TEXT,
    email           TEXT,
    avatar          TEXT,
    status          TEXT NOT NULL DEFAULT 'active',
    raw             JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_owner, provider_name)
);

CREATE TABLE IF NOT EXISTS iam_identity_groups (
    id              TEXT PRIMARY KEY,
    provider        TEXT NOT NULL,
    provider_owner  TEXT NOT NULL,
    provider_name   TEXT NOT NULL,
    display_name    TEXT,
    type            TEXT NOT NULL,
    parent_id       TEXT,
    raw             JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_owner, provider_name)
);

CREATE TABLE IF NOT EXISTS iam_identity_group_members (
    group_id        TEXT NOT NULL,
    subject_type    TEXT NOT NULL,
    subject_id      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(group_id, subject_type, subject_id)
);

CREATE TABLE IF NOT EXISTS iam_audit_logs (
    id              TEXT PRIMARY KEY,
    actor_type      TEXT NOT NULL,
    actor_id        TEXT NOT NULL,
    technical_actor TEXT,
    action          TEXT NOT NULL,
    resource_type   TEXT,
    resource_id     TEXT,
    result          TEXT NOT NULL,
    reason          TEXT,
    request_id      TEXT,
    trace_id        TEXT,
    ip              TEXT,
    user_agent      TEXT,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS iam_audit_logs;
DROP TABLE IF EXISTS iam_identity_group_members;
DROP TABLE IF EXISTS iam_identity_groups;
DROP TABLE IF EXISTS iam_identity_users;
