package data

import (
	"context"
	"strings"
	"time"

	"github.com/aisphereio/kernel/dbx"
)

const (
	StatusActive   = "active"
	StatusArchived = "archived"
	StatusDeleted  = "deleted"
	StatusPending  = "pending"
	StatusSynced   = "synced"
)

type ListOptions struct {
	OrgID        string
	ProjectID    string
	Type         string
	CapabilityID string
	ResourceType string
	ResourceID   string
	SubjectType  string
	SubjectID    string
	Status       string
	Q            string
	Page         int
	Size         int
}

type Page[T any] struct {
	Items   []T
	Total   int64
	Page    int
	Size    int
	HasMore bool
}

type ControlPlaneRepository interface {
	CreateProject(ctx context.Context, project *ProjectModel, outbox ...*OutboxEventModel) error
	UpsertProject(ctx context.Context, project *ProjectModel) error
	GetProject(ctx context.Context, id string) (*ProjectModel, error)
	ListProjects(ctx context.Context, opts ListOptions) (*Page[ProjectModel], error)
	ArchiveProject(ctx context.Context, id string) error

	UpsertCapability(ctx context.Context, capability *CapabilityModel) error
	ListCapabilities(ctx context.Context, opts ListOptions) ([]CapabilityModel, error)
	SetProjectCapability(ctx context.Context, pc *ProjectCapabilityModel) error
	ListProjectCapabilities(ctx context.Context, projectID string) ([]ProjectCapabilityModel, error)

	UpsertResourceType(ctx context.Context, rt *ResourceTypeModel) error
	GetResourceType(ctx context.Context, typ string) (*ResourceTypeModel, error)
	ListResourceTypes(ctx context.Context, opts ListOptions) ([]ResourceTypeModel, error)

	UpsertResource(ctx context.Context, resource *ResourceModel, outbox ...*OutboxEventModel) error
	GetResource(ctx context.Context, typ, id string) (*ResourceModel, error)
	ListResources(ctx context.Context, opts ListOptions) (*Page[ResourceModel], error)
	ArchiveResource(ctx context.Context, typ, id string) error

	BindResource(ctx context.Context, binding *ResourceBindingModel, outbox ...*OutboxEventModel) error
	ListResourceBindings(ctx context.Context, opts ListOptions) ([]ResourceBindingModel, error)
	BindExternalResource(ctx context.Context, binding *ExternalResourceBindingModel) error

	UpsertRoleTemplate(ctx context.Context, role *RoleTemplateModel) error
	ListRoleTemplates(ctx context.Context, resourceType string) ([]RoleTemplateModel, error)

	CreateGrant(ctx context.Context, grant *GrantModel, audit *GrantAuditModel, outbox ...*OutboxEventModel) error
	GetGrant(ctx context.Context, id string) (*GrantModel, error)
	RevokeGrant(ctx context.Context, id string, revokedAt time.Time, audit *GrantAuditModel, outbox ...*OutboxEventModel) error
	ListGrants(ctx context.Context, opts ListOptions) (*Page[GrantModel], error)

	CreateOutboxEvents(ctx context.Context, events ...*OutboxEventModel) error
	GetOutboxEvent(ctx context.Context, id string) (*OutboxEventModel, error)
	UpdateOutboxEvent(ctx context.Context, id string, columns map[string]any) error
	ListOutboxEvents(ctx context.Context, opts ListOptions) ([]OutboxEventModel, error)
}

type DBControlPlaneRepository struct {
	db dbx.DB
}

func NewControlPlaneRepository(db dbx.DB) *DBControlPlaneRepository {
	return &DBControlPlaneRepository{db: db}
}

func (r *DBControlPlaneRepository) CreateProject(ctx context.Context, project *ProjectModel, outbox ...*OutboxEventModel) error {
	return r.db.InTx(ctx, func(tx dbx.Tx) error {
		if err := tx.Create(ctx, project); err != nil {
			return err
		}
		return createOutbox(ctx, tx, outbox...)
	})
}

func (r *DBControlPlaneRepository) UpsertProject(ctx context.Context, project *ProjectModel) error {
	return r.db.SafeUpsert(ctx, project, []string{"display_name", "description", "status", "visibility", "labels_json", "annotations_json", "metadata_json", "updated_at"})
}

func (r *DBControlPlaneRepository) GetProject(ctx context.Context, id string) (*ProjectModel, error) {
	var out ProjectModel
	if err := r.db.FindOne(ctx, &out, "id = ?", id); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *DBControlPlaneRepository) ArchiveProject(ctx context.Context, id string) error {
	now := time.Now().UTC()
	return r.db.Update(ctx, &ProjectModel{}, "id = ?", []any{id}, map[string]any{"status": StatusArchived, "updated_at": now})
}

func (r *DBControlPlaneRepository) ListProjects(ctx context.Context, opts ListOptions) (*Page[ProjectModel], error) {
	var out []ProjectModel
	query, args := whereBuilder().eq("org_id", opts.OrgID).eq("status", opts.Status).likeAny([]string{"slug", "display_name"}, opts.Q).build()
	res, err := r.db.Paginate(ctx, &out, &ProjectModel{}, query, args, opts.Page, opts.Size)
	return pageFrom(out, res, err)
}

func (r *DBControlPlaneRepository) UpsertCapability(ctx context.Context, capability *CapabilityModel) error {
	return r.db.SafeUpsert(ctx, capability, []string{"name", "display_name", "owner_service", "status", "config_schema", "updated_at"})
}

func (r *DBControlPlaneRepository) ListCapabilities(ctx context.Context, opts ListOptions) ([]CapabilityModel, error) {
	var out []CapabilityModel
	query, args := whereBuilder().eq("status", opts.Status).likeAny([]string{"name", "display_name"}, opts.Q).build()
	return out, r.db.FindMany(ctx, &out, query, args...)
}

func (r *DBControlPlaneRepository) SetProjectCapability(ctx context.Context, pc *ProjectCapabilityModel) error {
	return r.db.SafeUpsert(ctx, pc, []string{"enabled", "config_json", "quota_json", "updated_at"})
}

func (r *DBControlPlaneRepository) ListProjectCapabilities(ctx context.Context, projectID string) ([]ProjectCapabilityModel, error) {
	var out []ProjectCapabilityModel
	return out, r.db.FindMany(ctx, &out, "project_id = ?", projectID)
}

func (r *DBControlPlaneRepository) UpsertResourceType(ctx context.Context, rt *ResourceTypeModel) error {
	return r.db.SafeUpsert(ctx, rt, []string{"capability_id", "owner_service", "parent_types_json", "grantable", "auditable", "spicedb_type", "relations_json", "permissions_json", "metadata_schema", "status", "updated_at"})
}

func (r *DBControlPlaneRepository) GetResourceType(ctx context.Context, typ string) (*ResourceTypeModel, error) {
	var out ResourceTypeModel
	if err := r.db.FindOne(ctx, &out, "type = ?", typ); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *DBControlPlaneRepository) ListResourceTypes(ctx context.Context, opts ListOptions) ([]ResourceTypeModel, error) {
	var out []ResourceTypeModel
	query, args := whereBuilder().eq("capability_id", opts.CapabilityID).eq("status", opts.Status).build()
	return out, r.db.FindMany(ctx, &out, query, args...)
}

func (r *DBControlPlaneRepository) UpsertResource(ctx context.Context, resource *ResourceModel, outbox ...*OutboxEventModel) error {
	return r.db.InTx(ctx, func(tx dbx.Tx) error {
		if err := tx.SafeUpsert(ctx, resource, []string{"org_id", "project_id", "parent_type", "parent_id", "owner_service", "owner_resource_id", "slug", "display_name", "path", "status", "visibility", "labels_json", "annotations_json", "metadata_json", "updated_at"}); err != nil {
			return err
		}
		return createOutbox(ctx, tx, outbox...)
	})
}

func (r *DBControlPlaneRepository) GetResource(ctx context.Context, typ, id string) (*ResourceModel, error) {
	var out ResourceModel
	if err := r.db.FindOne(ctx, &out, "type = ? AND id = ?", typ, id); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *DBControlPlaneRepository) ListResources(ctx context.Context, opts ListOptions) (*Page[ResourceModel], error) {
	var out []ResourceModel
	query, args := whereBuilder().eq("org_id", opts.OrgID).eq("project_id", opts.ProjectID).eq("type", opts.Type).eq("status", opts.Status).likeAny([]string{"slug", "display_name", "path"}, opts.Q).build()
	res, err := r.db.Paginate(ctx, &out, &ResourceModel{}, query, args, opts.Page, opts.Size)
	return pageFrom(out, res, err)
}

func (r *DBControlPlaneRepository) ArchiveResource(ctx context.Context, typ, id string) error {
	now := time.Now().UTC()
	return r.db.Update(ctx, &ResourceModel{}, "type = ? AND id = ?", []any{typ, id}, map[string]any{"status": StatusArchived, "updated_at": now})
}

func (r *DBControlPlaneRepository) BindResource(ctx context.Context, binding *ResourceBindingModel, outbox ...*OutboxEventModel) error {
	return r.db.InTx(ctx, func(tx dbx.Tx) error {
		if err := tx.SafeUpsert(ctx, binding, []string{"status", "updated_at"}); err != nil {
			return err
		}
		return createOutbox(ctx, tx, outbox...)
	})
}

func (r *DBControlPlaneRepository) ListResourceBindings(ctx context.Context, opts ListOptions) ([]ResourceBindingModel, error) {
	var out []ResourceBindingModel
	query, args := whereBuilder().eq("source_type", opts.ResourceType).eq("source_id", opts.ResourceID).eq("status", opts.Status).build()
	return out, r.db.FindMany(ctx, &out, query, args...)
}

func (r *DBControlPlaneRepository) BindExternalResource(ctx context.Context, binding *ExternalResourceBindingModel) error {
	return r.db.SafeUpsert(ctx, binding, []string{"resource_type", "resource_id", "external_path", "external_url", "sync_mode", "sync_status", "last_synced_at", "metadata_json", "updated_at"})
}

func (r *DBControlPlaneRepository) UpsertRoleTemplate(ctx context.Context, role *RoleTemplateModel) error {
	return r.db.SafeUpsert(ctx, role, []string{"display_name", "description", "relation", "built_in", "enabled", "sort_order", "metadata_json", "updated_at"})
}

func (r *DBControlPlaneRepository) ListRoleTemplates(ctx context.Context, resourceType string) ([]RoleTemplateModel, error) {
	var out []RoleTemplateModel
	query, args := whereBuilder().eq("resource_type", resourceType).eq("enabled", true).build()
	return out, r.db.FindMany(ctx, &out, query, args...)
}

func (r *DBControlPlaneRepository) CreateGrant(ctx context.Context, grant *GrantModel, audit *GrantAuditModel, outbox ...*OutboxEventModel) error {
	return r.db.InTx(ctx, func(tx dbx.Tx) error {
		if err := tx.Create(ctx, grant); err != nil {
			return err
		}
		if audit != nil {
			if err := tx.Create(ctx, audit); err != nil {
				return err
			}
		}
		return createOutbox(ctx, tx, outbox...)
	})
}

func (r *DBControlPlaneRepository) GetGrant(ctx context.Context, id string) (*GrantModel, error) {
	var out GrantModel
	if err := r.db.FindOne(ctx, &out, "id = ?", id); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *DBControlPlaneRepository) RevokeGrant(ctx context.Context, id string, revokedAt time.Time, audit *GrantAuditModel, outbox ...*OutboxEventModel) error {
	return r.db.InTx(ctx, func(tx dbx.Tx) error {
		if err := tx.Update(ctx, &GrantModel{}, "id = ? AND revoked_at IS NULL", []any{id}, map[string]any{"revoked_at": revokedAt}); err != nil {
			return err
		}
		if audit != nil {
			if err := tx.Create(ctx, audit); err != nil {
				return err
			}
		}
		return createOutbox(ctx, tx, outbox...)
	})
}

func (r *DBControlPlaneRepository) ListGrants(ctx context.Context, opts ListOptions) (*Page[GrantModel], error) {
	var out []GrantModel
	query, args := whereBuilder().eq("resource_type", opts.ResourceType).eq("resource_id", opts.ResourceID).eq("subject_type", opts.SubjectType).eq("subject_id", opts.SubjectID).build()
	res, err := r.db.Paginate(ctx, &out, &GrantModel{}, query, args, opts.Page, opts.Size)
	return pageFrom(out, res, err)
}

func (r *DBControlPlaneRepository) CreateOutboxEvents(ctx context.Context, events ...*OutboxEventModel) error {
	return r.db.InTx(ctx, func(tx dbx.Tx) error {
		return createOutbox(ctx, tx, events...)
	})
}

func (r *DBControlPlaneRepository) GetOutboxEvent(ctx context.Context, id string) (*OutboxEventModel, error) {
	var out OutboxEventModel
	if err := r.db.FindOne(ctx, &out, "id = ?", id); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *DBControlPlaneRepository) UpdateOutboxEvent(ctx context.Context, id string, columns map[string]any) error {
	if columns == nil {
		columns = map[string]any{}
	}
	columns["updated_at"] = time.Now().UTC()
	return r.db.Update(ctx, &OutboxEventModel{}, "id = ?", []any{id}, columns)
}

func (r *DBControlPlaneRepository) ListOutboxEvents(ctx context.Context, opts ListOptions) ([]OutboxEventModel, error) {
	var out []OutboxEventModel
	query, args := whereBuilder().eq("topic", opts.Type).eq("status", opts.Status).build()
	return out, r.db.FindMany(ctx, &out, query, args...)
}

// ─── Local Users ───────────────────────────────────────────────────────

type LocalUserRepository interface {
	ListUsers(ctx context.Context) ([]LocalUserModel, error)
	GetUser(ctx context.Context, username string) (*LocalUserModel, error)
	SaveUser(ctx context.Context, user *LocalUserModel) error
	DeleteUser(ctx context.Context, username string) error
}

type DBLocalUserRepository struct {
	db dbx.DB
}

func NewLocalUserRepository(db dbx.DB) *DBLocalUserRepository {
	return &DBLocalUserRepository{db: db}
}

func (r *DBLocalUserRepository) ListUsers(ctx context.Context) ([]LocalUserModel, error) {
	var out []LocalUserModel
	return out, r.db.FindMany(ctx, &out, "true ORDER BY username ASC")
}

func (r *DBLocalUserRepository) GetUser(ctx context.Context, username string) (*LocalUserModel, error) {
	var out LocalUserModel
	if err := r.db.FindOne(ctx, &out, "username = ?", username); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *DBLocalUserRepository) SaveUser(ctx context.Context, user *LocalUserModel) error {
	return r.db.SafeUpsert(ctx, user, []string{"subject_id", "subject_type", "display_name", "email", "organization", "roles_json", "permissions_json", "namespaces_json", "password_hash", "disabled", "updated_at"})
}

func (r *DBLocalUserRepository) DeleteUser(ctx context.Context, username string) error {
	return r.db.Delete(ctx, &LocalUserModel{}, "username = ?", username)
}

type outboxCreator interface {
	Create(context.Context, any) error
}

func createOutbox(ctx context.Context, store outboxCreator, events ...*OutboxEventModel) error {
	for _, event := range events {
		if event == nil {
			continue
		}
		if err := store.Create(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type where struct {
	parts []string
	args  []any
}

func whereBuilder() *where { return &where{} }

func (w *where) eq(col string, value any) *where {
	if value == nil {
		return w
	}
	if v, ok := value.(string); ok && strings.TrimSpace(v) == "" {
		return w
	}
	w.parts = append(w.parts, col+" = ?")
	w.args = append(w.args, value)
	return w
}

func (w *where) likeAny(cols []string, q string) *where {
	q = strings.TrimSpace(q)
	if q == "" || len(cols) == 0 {
		return w
	}
	clauses := make([]string, 0, len(cols))
	for _, col := range cols {
		clauses = append(clauses, col+" ILIKE ?")
		w.args = append(w.args, "%"+q+"%")
	}
	w.parts = append(w.parts, "("+strings.Join(clauses, " OR ")+")")
	return w
}

func (w *where) build() (any, []any) {
	if len(w.parts) == 0 {
		return nil, nil
	}
	return strings.Join(w.parts, " AND "), w.args
}

func pageFrom[T any](items []T, res *dbx.PageResult, err error) (*Page[T], error) {
	if err != nil {
		return nil, err
	}
	return &Page[T]{Items: items, Total: res.Total, Page: res.Page, Size: res.Size, HasMore: res.HasMore}, nil
}
