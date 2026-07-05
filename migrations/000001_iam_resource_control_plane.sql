-- +goose Up
CREATE TABLE IF NOT EXISTS iam_organizations (
  id text PRIMARY KEY,
  slug text NOT NULL UNIQUE,
  display_name text NOT NULL,
  status text NOT NULL,
  casdoor_org text,
  plan text,
  region text,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS iam_projects (
  id text PRIMARY KEY,
  org_id text NOT NULL REFERENCES iam_organizations(id),
  slug text NOT NULL,
  display_name text NOT NULL,
  description text,
  status text NOT NULL,
  visibility text NOT NULL DEFAULT 'private',
  labels_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  annotations_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT uq_iam_project_org_slug UNIQUE (org_id, slug)
);
CREATE INDEX IF NOT EXISTS idx_iam_projects_org_id ON iam_projects(org_id);
CREATE INDEX IF NOT EXISTS idx_iam_projects_status ON iam_projects(status);
CREATE INDEX IF NOT EXISTS idx_iam_projects_deleted_at ON iam_projects(deleted_at);

CREATE TABLE IF NOT EXISTS iam_capabilities (
  id text PRIMARY KEY,
  name text NOT NULL UNIQUE,
  display_name text NOT NULL,
  owner_service text NOT NULL,
  status text NOT NULL,
  config_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_iam_capabilities_status ON iam_capabilities(status);

CREATE TABLE IF NOT EXISTS iam_project_capabilities (
  project_id text NOT NULL REFERENCES iam_projects(id),
  capability_id text NOT NULL REFERENCES iam_capabilities(id),
  enabled boolean NOT NULL DEFAULT true,
  config_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  quota_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, capability_id)
);

CREATE TABLE IF NOT EXISTS iam_resource_types (
  type text PRIMARY KEY,
  capability_id text NOT NULL REFERENCES iam_capabilities(id),
  owner_service text NOT NULL,
  parent_types_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  grantable boolean NOT NULL DEFAULT true,
  auditable boolean NOT NULL DEFAULT true,
  spicedb_type text NOT NULL,
  relations_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  permissions_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  metadata_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_iam_resource_types_capability_id ON iam_resource_types(capability_id);
CREATE INDEX IF NOT EXISTS idx_iam_resource_types_status ON iam_resource_types(status);

CREATE TABLE IF NOT EXISTS iam_resources (
  id text NOT NULL,
  type text NOT NULL REFERENCES iam_resource_types(type),
  org_id text NOT NULL REFERENCES iam_organizations(id),
  project_id text REFERENCES iam_projects(id),
  parent_type text,
  parent_id text,
  owner_service text NOT NULL,
  owner_resource_id text NOT NULL,
  slug text,
  display_name text,
  path text,
  status text NOT NULL,
  visibility text NOT NULL DEFAULT 'private',
  labels_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  annotations_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  PRIMARY KEY (type, id)
);
CREATE INDEX IF NOT EXISTS idx_iam_resources_type ON iam_resources(type);
CREATE INDEX IF NOT EXISTS idx_iam_resources_org_id ON iam_resources(org_id);
CREATE INDEX IF NOT EXISTS idx_iam_resources_project_id ON iam_resources(project_id);
CREATE INDEX IF NOT EXISTS idx_iam_resources_parent ON iam_resources(parent_type, parent_id);
CREATE INDEX IF NOT EXISTS idx_iam_resources_owner ON iam_resources(owner_service, owner_resource_id);
CREATE INDEX IF NOT EXISTS idx_iam_resources_slug ON iam_resources(slug);
CREATE INDEX IF NOT EXISTS idx_iam_resources_path ON iam_resources(path);
CREATE INDEX IF NOT EXISTS idx_iam_resources_status ON iam_resources(status);
CREATE INDEX IF NOT EXISTS idx_iam_resources_deleted_at ON iam_resources(deleted_at);

CREATE TABLE IF NOT EXISTS iam_resource_bindings (
  id text PRIMARY KEY,
  source_type text NOT NULL,
  source_id text NOT NULL,
  relation text NOT NULL,
  target_type text NOT NULL,
  target_id text NOT NULL,
  status text NOT NULL,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_iam_resource_binding UNIQUE (source_type, source_id, relation, target_type, target_id)
);
CREATE INDEX IF NOT EXISTS idx_iam_resource_bindings_source ON iam_resource_bindings(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_iam_resource_bindings_target ON iam_resource_bindings(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_iam_resource_bindings_status ON iam_resource_bindings(status);

CREATE TABLE IF NOT EXISTS iam_external_resource_bindings (
  id text PRIMARY KEY,
  resource_type text NOT NULL,
  resource_id text NOT NULL,
  provider text NOT NULL,
  external_type text NOT NULL,
  external_id text NOT NULL,
  external_path text,
  external_url text,
  sync_mode text,
  sync_status text,
  last_synced_at timestamptz,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_iam_external_resource_binding UNIQUE (provider, external_type, external_id)
);
CREATE INDEX IF NOT EXISTS idx_iam_external_bindings_resource ON iam_external_resource_bindings(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_iam_external_bindings_sync_status ON iam_external_resource_bindings(sync_status);

CREATE TABLE IF NOT EXISTS iam_role_templates (
  id text PRIMARY KEY,
  resource_type text NOT NULL,
  role_key text NOT NULL,
  display_name text NOT NULL,
  description text,
  relation text NOT NULL,
  built_in boolean NOT NULL DEFAULT false,
  enabled boolean NOT NULL DEFAULT true,
  sort_order integer NOT NULL DEFAULT 0,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_iam_role_template_resource_role UNIQUE (resource_type, role_key)
);
CREATE INDEX IF NOT EXISTS idx_iam_role_templates_resource_type ON iam_role_templates(resource_type);
CREATE INDEX IF NOT EXISTS idx_iam_role_templates_enabled ON iam_role_templates(enabled);

CREATE TABLE IF NOT EXISTS iam_grants (
  id text PRIMARY KEY,
  resource_type text NOT NULL,
  resource_id text NOT NULL,
  role_key text NOT NULL,
  relation text NOT NULL,
  subject_type text NOT NULL,
  subject_id text NOT NULL,
  subject_relation text,
  source text NOT NULL,
  reason text,
  expires_at timestamptz,
  created_by_type text,
  created_by_id text,
  created_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_iam_grants_resource ON iam_grants(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_iam_grants_subject ON iam_grants(subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_iam_grants_source ON iam_grants(source);
CREATE INDEX IF NOT EXISTS idx_iam_grants_expires_at ON iam_grants(expires_at);
CREATE INDEX IF NOT EXISTS idx_iam_grants_revoked_at ON iam_grants(revoked_at);

CREATE TABLE IF NOT EXISTS iam_grant_audits (
  id text PRIMARY KEY,
  grant_id text,
  action text NOT NULL,
  resource_type text NOT NULL,
  resource_id text NOT NULL,
  relation text NOT NULL,
  subject_type text NOT NULL,
  subject_id text NOT NULL,
  subject_relation text,
  actor_type text,
  actor_id text,
  reason text,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_iam_grant_audits_grant_id ON iam_grant_audits(grant_id);
CREATE INDEX IF NOT EXISTS idx_iam_grant_audits_action ON iam_grant_audits(action);
CREATE INDEX IF NOT EXISTS idx_iam_grant_audits_resource ON iam_grant_audits(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_iam_grant_audits_subject ON iam_grant_audits(subject_type, subject_id);

CREATE TABLE IF NOT EXISTS iam_outbox_events (
  id text PRIMARY KEY,
  topic text NOT NULL,
  aggregate_type text NOT NULL,
  aggregate_id text NOT NULL,
  payload_json jsonb NOT NULL,
  status text NOT NULL,
  retry_count integer NOT NULL DEFAULT 0,
  next_run_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_iam_outbox_events_topic ON iam_outbox_events(topic);
CREATE INDEX IF NOT EXISTS idx_iam_outbox_events_aggregate ON iam_outbox_events(aggregate_type, aggregate_id);
CREATE INDEX IF NOT EXISTS idx_iam_outbox_events_status ON iam_outbox_events(status);
CREATE INDEX IF NOT EXISTS idx_iam_outbox_events_next_run_at ON iam_outbox_events(next_run_at);

-- +goose Down
DROP TABLE IF EXISTS iam_outbox_events;
DROP TABLE IF EXISTS iam_grant_audits;
DROP TABLE IF EXISTS iam_grants;
DROP TABLE IF EXISTS iam_role_templates;
DROP TABLE IF EXISTS iam_external_resource_bindings;
DROP TABLE IF EXISTS iam_resource_bindings;
DROP TABLE IF EXISTS iam_resources;
DROP TABLE IF EXISTS iam_resource_types;
DROP TABLE IF EXISTS iam_project_capabilities;
DROP TABLE IF EXISTS iam_capabilities;
DROP TABLE IF EXISTS iam_projects;
DROP TABLE IF EXISTS iam_organizations;
