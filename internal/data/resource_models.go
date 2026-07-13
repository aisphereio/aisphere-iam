package data

import "time"

// Resource control plane database models.
//
// These structs intentionally use kernel/dbx + GORM conventions instead of
// introducing another data-access stack. They are resource projections and
// control-plane records only; domain resources such as skills, repositories,
// agents, and sandboxes remain owned by their domain services.

type ProjectModel struct {
	ID              string     `gorm:"column:id;primaryKey" json:"id"`
	OrgID           string     `gorm:"column:org_id;not null;index;uniqueIndex:idx_iam_project_org_slug" json:"org_id"`
	Slug            string     `gorm:"column:slug;not null;uniqueIndex:idx_iam_project_org_slug" json:"slug"`
	DisplayName     string     `gorm:"column:display_name;not null" json:"display_name"`
	Description     string     `gorm:"column:description" json:"description"`
	Status          string     `gorm:"column:status;not null;index" json:"status"`
	Visibility      string     `gorm:"column:visibility;not null;default:'private'" json:"visibility"`
	LabelsJSON      string     `gorm:"column:labels_json;type:jsonb;default:'{}'" json:"labels_json"`
	AnnotationsJSON string     `gorm:"column:annotations_json;type:jsonb;default:'{}'" json:"annotations_json"`
	CreatedBy       string     `gorm:"column:created_by" json:"created_by"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

func (ProjectModel) TableName() string { return "iam_projects" }

type CapabilityModel struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	Name         string    `gorm:"column:name;not null;uniqueIndex" json:"name"`
	DisplayName  string    `gorm:"column:display_name;not null" json:"display_name"`
	OwnerService string    `gorm:"column:owner_service;not null" json:"owner_service"`
	Status       string    `gorm:"column:status;not null;index" json:"status"`
	ConfigSchema string    `gorm:"column:config_schema;type:jsonb;default:'{}'" json:"config_schema"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (CapabilityModel) TableName() string { return "iam_capabilities" }

type ProjectCapabilityModel struct {
	ProjectID    string    `gorm:"column:project_id;primaryKey" json:"project_id"`
	CapabilityID string    `gorm:"column:capability_id;primaryKey" json:"capability_id"`
	Enabled      bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	ConfigJSON   string    `gorm:"column:config_json;type:jsonb;default:'{}'" json:"config_json"`
	QuotaJSON    string    `gorm:"column:quota_json;type:jsonb;default:'{}'" json:"quota_json"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ProjectCapabilityModel) TableName() string { return "iam_project_capabilities" }

type ResourceTypeModel struct {
	Type            string    `gorm:"column:type;primaryKey" json:"type"`
	CapabilityID    string    `gorm:"column:capability_id;not null;index" json:"capability_id"`
	OwnerService    string    `gorm:"column:owner_service;not null" json:"owner_service"`
	ParentTypesJSON string    `gorm:"column:parent_types_json;type:jsonb;default:'[]'" json:"parent_types_json"`
	Grantable       bool      `gorm:"column:grantable;not null;default:true" json:"grantable"`
	Auditable       bool      `gorm:"column:auditable;not null;default:true" json:"auditable"`
	SpiceDBType     string    `gorm:"column:spicedb_type;not null" json:"spicedb_type"`
	RelationsJSON   string    `gorm:"column:relations_json;type:jsonb;default:'[]'" json:"relations_json"`
	PermissionsJSON string    `gorm:"column:permissions_json;type:jsonb;default:'[]'" json:"permissions_json"`
	MetadataSchema  string    `gorm:"column:metadata_schema;type:jsonb;default:'{}'" json:"metadata_schema"`
	Status          string    `gorm:"column:status;not null;index" json:"status"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ResourceTypeModel) TableName() string { return "iam_resource_types" }

type ResourceModel struct {
	Type            string     `gorm:"column:type;primaryKey" json:"type"`
	ID              string     `gorm:"column:id;primaryKey" json:"id"`
	OrgID           string     `gorm:"column:org_id;not null;index" json:"org_id"`
	ProjectID       string     `gorm:"column:project_id;index" json:"project_id"`
	ParentType      string     `gorm:"column:parent_type;index:idx_iam_resource_parent" json:"parent_type"`
	ParentID        string     `gorm:"column:parent_id;index:idx_iam_resource_parent" json:"parent_id"`
	OwnerService    string     `gorm:"column:owner_service;not null;index" json:"owner_service"`
	OwnerResourceID string     `gorm:"column:owner_resource_id;not null;index" json:"owner_resource_id"`
	Slug            string     `gorm:"column:slug;index" json:"slug"`
	DisplayName     string     `gorm:"column:display_name" json:"display_name"`
	Path            string     `gorm:"column:path;index" json:"path"`
	Status          string     `gorm:"column:status;not null;index" json:"status"`
	Visibility      string     `gorm:"column:visibility;not null;default:'private'" json:"visibility"`
	LabelsJSON      string     `gorm:"column:labels_json;type:jsonb;default:'{}'" json:"labels_json"`
	AnnotationsJSON string     `gorm:"column:annotations_json;type:jsonb;default:'{}'" json:"annotations_json"`
	MetadataJSON    string     `gorm:"column:metadata_json;type:jsonb;default:'{}'" json:"metadata_json"`
	CreatedBy       string     `gorm:"column:created_by" json:"created_by"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

func (ResourceModel) TableName() string { return "iam_resources" }

type ResourceBindingModel struct {
	ID         string    `gorm:"column:id;primaryKey" json:"id"`
	SourceType string    `gorm:"column:source_type;not null;uniqueIndex:idx_iam_resource_binding_unique" json:"source_type"`
	SourceID   string    `gorm:"column:source_id;not null;uniqueIndex:idx_iam_resource_binding_unique" json:"source_id"`
	Relation   string    `gorm:"column:relation;not null;uniqueIndex:idx_iam_resource_binding_unique" json:"relation"`
	TargetType string    `gorm:"column:target_type;not null;uniqueIndex:idx_iam_resource_binding_unique" json:"target_type"`
	TargetID   string    `gorm:"column:target_id;not null;uniqueIndex:idx_iam_resource_binding_unique" json:"target_id"`
	Status     string    `gorm:"column:status;not null;index" json:"status"`
	CreatedBy  string    `gorm:"column:created_by" json:"created_by"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ResourceBindingModel) TableName() string { return "iam_resource_bindings" }

type ExternalResourceBindingModel struct {
	ID           string     `gorm:"column:id;primaryKey" json:"id"`
	ResourceType string     `gorm:"column:resource_type;not null;index" json:"resource_type"`
	ResourceID   string     `gorm:"column:resource_id;not null;index" json:"resource_id"`
	Provider     string     `gorm:"column:provider;not null;uniqueIndex:idx_iam_external_binding_unique" json:"provider"`
	ExternalType string     `gorm:"column:external_type;not null;uniqueIndex:idx_iam_external_binding_unique" json:"external_type"`
	ExternalID   string     `gorm:"column:external_id;not null;uniqueIndex:idx_iam_external_binding_unique" json:"external_id"`
	ExternalPath string     `gorm:"column:external_path" json:"external_path"`
	ExternalURL  string     `gorm:"column:external_url" json:"external_url"`
	SyncMode     string     `gorm:"column:sync_mode" json:"sync_mode"`
	SyncStatus   string     `gorm:"column:sync_status;index" json:"sync_status"`
	LastSyncedAt *time.Time `gorm:"column:last_synced_at" json:"last_synced_at,omitempty"`
	MetadataJSON string     `gorm:"column:metadata_json;type:jsonb;default:'{}'" json:"metadata_json"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ExternalResourceBindingModel) TableName() string { return "iam_external_resource_bindings" }

type RoleTemplateModel struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	ResourceType string    `gorm:"column:resource_type;not null;uniqueIndex:idx_iam_role_template_resource_role" json:"resource_type"`
	RoleKey      string    `gorm:"column:role_key;not null;uniqueIndex:idx_iam_role_template_resource_role" json:"role_key"`
	DisplayName  string    `gorm:"column:display_name;not null" json:"display_name"`
	Description  string    `gorm:"column:description" json:"description"`
	Relation     string    `gorm:"column:relation;not null" json:"relation"`
	BuiltIn      bool      `gorm:"column:built_in;not null;default:false" json:"built_in"`
	Enabled      bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	SortOrder    int       `gorm:"column:sort_order" json:"sort_order"`
	MetadataJSON string    `gorm:"column:metadata_json;type:jsonb;default:'{}'" json:"metadata_json"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (RoleTemplateModel) TableName() string { return "iam_role_templates" }

type GrantModel struct {
	ID              string     `gorm:"column:id;primaryKey" json:"id"`
	ResourceType    string     `gorm:"column:resource_type;not null;index:idx_iam_grant_resource" json:"resource_type"`
	ResourceID      string     `gorm:"column:resource_id;not null;index:idx_iam_grant_resource" json:"resource_id"`
	RoleKey         string     `gorm:"column:role_key;not null" json:"role_key"`
	Relation        string     `gorm:"column:relation;not null" json:"relation"`
	SubjectType     string     `gorm:"column:subject_type;not null;index:idx_iam_grant_subject" json:"subject_type"`
	SubjectID       string     `gorm:"column:subject_id;not null;index:idx_iam_grant_subject" json:"subject_id"`
	SubjectRelation string     `gorm:"column:subject_relation" json:"subject_relation"`
	Source          string     `gorm:"column:source;not null;index" json:"source"`
	Reason          string     `gorm:"column:reason" json:"reason"`
	ExpiresAt       *time.Time `gorm:"column:expires_at;index" json:"expires_at,omitempty"`
	CreatedByType   string     `gorm:"column:created_by_type" json:"created_by_type"`
	CreatedByID     string     `gorm:"column:created_by_id" json:"created_by_id"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	RevokedAt       *time.Time `gorm:"column:revoked_at;index" json:"revoked_at,omitempty"`
}

func (GrantModel) TableName() string { return "iam_grants" }

type GrantAuditModel struct {
	ID              string    `gorm:"column:id;primaryKey" json:"id"`
	GrantID         string    `gorm:"column:grant_id;index" json:"grant_id"`
	Action          string    `gorm:"column:action;not null;index" json:"action"`
	ResourceType    string    `gorm:"column:resource_type;not null;index" json:"resource_type"`
	ResourceID      string    `gorm:"column:resource_id;not null;index" json:"resource_id"`
	Relation        string    `gorm:"column:relation;not null" json:"relation"`
	SubjectType     string    `gorm:"column:subject_type;not null;index" json:"subject_type"`
	SubjectID       string    `gorm:"column:subject_id;not null;index" json:"subject_id"`
	SubjectRelation string    `gorm:"column:subject_relation" json:"subject_relation"`
	ActorType       string    `gorm:"column:actor_type" json:"actor_type"`
	ActorID         string    `gorm:"column:actor_id" json:"actor_id"`
	Reason          string    `gorm:"column:reason" json:"reason"`
	MetadataJSON    string    `gorm:"column:metadata_json;type:jsonb;default:'{}'" json:"metadata_json"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (GrantAuditModel) TableName() string { return "iam_grant_audits" }

type OutboxEventModel struct {
	ID            string     `gorm:"column:id;primaryKey" json:"id"`
	Topic         string     `gorm:"column:topic;not null;index" json:"topic"`
	AggregateType string     `gorm:"column:aggregate_type;not null;index" json:"aggregate_type"`
	AggregateID   string     `gorm:"column:aggregate_id;not null;index" json:"aggregate_id"`
	PayloadJSON   string     `gorm:"column:payload_json;type:jsonb;not null" json:"payload_json"`
	Status        string     `gorm:"column:status;not null;index" json:"status"`
	RetryCount    int        `gorm:"column:retry_count;not null;default:0" json:"retry_count"`
	NextRunAt     *time.Time `gorm:"column:next_run_at;index" json:"next_run_at,omitempty"`
	LastError     string     `gorm:"column:last_error" json:"last_error"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (OutboxEventModel) TableName() string { return "iam_outbox_events" }

type LocalUserModel struct {
	Username        string    `gorm:"column:username;primaryKey" json:"username"`
	SubjectID       string    `gorm:"column:subject_id" json:"subject_id"`
	SubjectType     string    `gorm:"column:subject_type;not null;default:'human'" json:"subject_type"`
	DisplayName     string    `gorm:"column:display_name" json:"display_name"`
	Email           string    `gorm:"column:email" json:"email"`
	Organization    string    `gorm:"column:organization" json:"organization"`
	RolesJSON       string    `gorm:"column:roles_json;type:jsonb;default:'[]'" json:"roles_json"`
	PermissionsJSON string    `gorm:"column:permissions_json;type:jsonb;default:'[]'" json:"permissions_json"`
	NamespacesJSON  string    `gorm:"column:namespaces_json;type:jsonb;default:'[]'" json:"namespaces_json"`
	PasswordHash    string    `gorm:"column:password_hash" json:"-"`
	Disabled        bool      `gorm:"column:disabled;not null;default:false" json:"disabled"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (LocalUserModel) TableName() string { return "iam_local_users" }

func ControlPlaneModels() []any {
	return []any{
		&ProjectModel{},
		&CapabilityModel{},
		&ProjectCapabilityModel{},
		&ResourceTypeModel{},
		&ResourceModel{},
		&ResourceBindingModel{},
		&ExternalResourceBindingModel{},
		&RoleTemplateModel{},
		&GrantModel{},
		&GrantAuditModel{},
		&OutboxEventModel{},
		&LocalUserModel{},
	}
}
