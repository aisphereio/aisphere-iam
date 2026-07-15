package data

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aisphereio/kernel/dbx"
)

var ErrRoleVersionConflict = errors.New("iam data: role template version conflict")

const (
	StatusActive   = "active"
	StatusArchived = "archived"
	StatusDeleted  = "deleted"
	StatusPending  = "pending"
	StatusSynced   = "synced"
	// StatusSubmitted / StatusProjecting / StatusFailed mirror the projection
	// manager's lifecycle for iam_outbox_events and are used by the retry worker.
	StatusSubmitted = "submitted"
	StatusProjecting = "projecting"
	StatusFailed     = "failed"

	// StatusDead marks an outbox event that has exhausted MaxOutboxRetries and
	// must no longer be retried automatically. It requires manual intervention.
	StatusDead = "dead"

	// MaxOutboxRetries caps automatic retry attempts for iam_outbox_events.
	// Once exceeded the event is dead-lettered (StatusDead) so the retry worker
	// stops spinning on a permanently failing saga.
	MaxOutboxRetries = 10
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
	Relation     string
	RoleKey      string
	Source       string
	Active       *bool
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
		GetProject(ctx context.Context, id, orgID string) (*ProjectModel, error)
		ListProjects(ctx context.Context, opts ListOptions) (*Page[ProjectModel], error)
		ArchiveProject(ctx context.Context, id, orgID string) error

	UpsertCapability(ctx context.Context, capability *CapabilityModel) error
	ListCapabilities(ctx context.Context, opts ListOptions) ([]CapabilityModel, error)
	SetProjectCapability(ctx context.Context, pc *ProjectCapabilityModel) error
	ListProjectCapabilities(ctx context.Context, projectID string) ([]ProjectCapabilityModel, error)

	UpsertResourceType(ctx context.Context, rt *ResourceTypeModel) error
	GetResourceType(ctx context.Context, typ string) (*ResourceTypeModel, error)
	ListResourceTypes(ctx context.Context, opts ListOptions) ([]ResourceTypeModel, error)

UpsertResource(ctx context.Context, resource *ResourceModel, outbox ...*OutboxEventModel) error
		GetResource(ctx context.Context, typ, id, orgID string) (*ResourceModel, error)
		ListResources(ctx context.Context, opts ListOptions) (*Page[ResourceModel], error)
		ArchiveResource(ctx context.Context, typ, id, orgID string) error
		DeleteResource(ctx context.Context, typ, id, orgID string) error

BindResource(ctx context.Context, binding *ResourceBindingModel, outbox ...*OutboxEventModel) error
		ListResourceBindings(ctx context.Context, opts ListOptions) ([]ResourceBindingModel, error)
		UnbindResource(ctx context.Context, bindingID, orgID string) error
		BindExternalResource(ctx context.Context, binding *ExternalResourceBindingModel) error
		ListExternalResourceBindings(ctx context.Context, opts ListOptions) ([]ExternalResourceBindingModel, error)

	UpsertRoleTemplate(ctx context.Context, role *RoleTemplateModel) error
	SaveRoleTemplate(ctx context.Context, role *RoleTemplateModel, audit *RoleTemplateAuditModel, outbox ...*OutboxEventModel) error
	GetRoleTemplate(ctx context.Context, id string) (*RoleTemplateModel, error)
	UpdateRoleTemplate(ctx context.Context, role *RoleTemplateModel, expectedVersion int64, audit *RoleTemplateAuditModel, outbox ...*OutboxEventModel) error
	CountActiveGrantsByRole(ctx context.Context, roleTemplateID string, at time.Time) (int64, error)
	ListRoleTemplates(ctx context.Context, resourceType string) ([]RoleTemplateModel, error)

CreateGrant(ctx context.Context, grant *GrantModel, audit *GrantAuditModel, outbox ...*OutboxEventModel) error
		GetGrant(ctx context.Context, id, orgID string) (*GrantModel, error)
		RevokeGrant(ctx context.Context, id, orgID string, revokedAt time.Time, audit *GrantAuditModel, outbox ...*OutboxEventModel) error
		ListGrants(ctx context.Context, opts ListOptions) (*Page[GrantModel], error)
		ListDueExpiringGrants(ctx context.Context, limit int) ([]GrantModel, error)

	CreateOutboxEvents(ctx context.Context, events ...*OutboxEventModel) error
	GetOutboxEvent(ctx context.Context, id string) (*OutboxEventModel, error)
	UpdateOutboxEvent(ctx context.Context, id string, columns map[string]any) error
	ListOutboxEvents(ctx context.Context, opts ListOptions) ([]OutboxEventModel, error)
	ListOutboxEventsForRetry(ctx context.Context, limit int) ([]OutboxEventModel, error)
	IncrementOutboxRetry(ctx context.Context, id string) (*OutboxEventModel, error)
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

func (r *DBControlPlaneRepository) GetProject(ctx context.Context, id, orgID string) (*ProjectModel, error) {
		var out ProjectModel
		if err := r.db.FindOne(ctx, &out, "id = ? AND org_id = ?", id, orgID); err != nil {
			return nil, err
		}
		return &out, nil
	}

func (r *DBControlPlaneRepository) ArchiveProject(ctx context.Context, id, orgID string) error {
		now := time.Now().UTC()
		return r.db.Update(ctx, &ProjectModel{}, "id = ? AND org_id = ?", []any{id, orgID}, map[string]any{"status": StatusArchived, "updated_at": now})
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

func (r *DBControlPlaneRepository) GetResource(ctx context.Context, typ, id, orgID string) (*ResourceModel, error) {
		var out ResourceModel
		if err := r.db.FindOne(ctx, &out, "type = ? AND id = ? AND org_id = ?", typ, id, orgID); err != nil {
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

func (r *DBControlPlaneRepository) ArchiveResource(ctx context.Context, typ, id, orgID string) error {
		now := time.Now().UTC()
		return r.db.Update(ctx, &ResourceModel{}, "type = ? AND id = ? AND org_id = ?", []any{typ, id, orgID}, map[string]any{"status": StatusArchived, "updated_at": now})
	}

func (r *DBControlPlaneRepository) DeleteResource(ctx context.Context, typ, id, orgID string) error {
		now := time.Now().UTC()
		return r.db.Update(ctx, &ResourceModel{}, "type = ? AND id = ? AND org_id = ?", []any{typ, id, orgID}, map[string]any{"status": StatusDeleted, "updated_at": now})
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
		query, args := whereBuilder().eq("org_id", opts.OrgID).eq("source_type", opts.ResourceType).eq("source_id", opts.ResourceID).eq("status", opts.Status).build()
		return out, r.db.FindMany(ctx, &out, query, args...)
	}

func (r *DBControlPlaneRepository) UnbindResource(ctx context.Context, bindingID, orgID string) error {
		return r.db.Delete(ctx, &ResourceBindingModel{}, "id = ? AND org_id = ?", bindingID, orgID)
	}

func (r *DBControlPlaneRepository) BindExternalResource(ctx context.Context, binding *ExternalResourceBindingModel) error {
	return r.db.SafeUpsert(ctx, binding, []string{"resource_type", "resource_id", "external_path", "external_url", "sync_mode", "sync_status", "last_synced_at", "metadata_json", "updated_at"})
}

func (r *DBControlPlaneRepository) ListExternalResourceBindings(ctx context.Context, opts ListOptions) ([]ExternalResourceBindingModel, error) {
		var out []ExternalResourceBindingModel
		query, args := whereBuilder().eq("org_id", opts.OrgID).eq("resource_type", opts.ResourceType).eq("resource_id", opts.ResourceID).eq("provider", opts.Type).eq("sync_status", opts.Status).build()
		return out, r.db.FindMany(ctx, &out, query, args...)
	}

func (r *DBControlPlaneRepository) UpsertRoleTemplate(ctx context.Context, role *RoleTemplateModel) error {
	return r.db.SafeUpsert(ctx, role, []string{"display_name", "description", "relation", "built_in", "enabled", "sort_order", "metadata_json", "version", "updated_at"})
}

func (r *DBControlPlaneRepository) SaveRoleTemplate(ctx context.Context, role *RoleTemplateModel, audit *RoleTemplateAuditModel, outbox ...*OutboxEventModel) error {
	return r.db.InTx(ctx, func(tx dbx.Tx) error {
		if err := tx.SafeUpsert(ctx, role, []string{"display_name", "description", "relation", "built_in", "enabled", "sort_order", "metadata_json", "version", "updated_at"}); err != nil {
			return err
		}
		if err := replaceRolePermissions(ctx, tx, role); err != nil {
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

func (r *DBControlPlaneRepository) GetRoleTemplate(ctx context.Context, id string) (*RoleTemplateModel, error) {
	var out RoleTemplateModel
	if err := r.db.FindOne(ctx, &out, "id = ?", id); err != nil {
		return nil, err
	}
	if err := loadRolePermissions(ctx, r.db, &out); err != nil {
		return nil, err
	}
	count, err := r.CountActiveGrantsByRole(ctx, out.ID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	out.ActiveGrantCount = count
	return &out, nil
}

func (r *DBControlPlaneRepository) UpdateRoleTemplate(ctx context.Context, role *RoleTemplateModel, expectedVersion int64, audit *RoleTemplateAuditModel, outbox ...*OutboxEventModel) error {
	return r.db.InTx(ctx, func(tx dbx.Tx) error {
		role.Version = expectedVersion + 1
		result := tx.GORM(ctx).Model(&RoleTemplateModel{}).Where("id = ? AND version = ?", role.ID, expectedVersion).Updates(map[string]any{
			"display_name": role.DisplayName, "description": role.Description, "enabled": role.Enabled,
			"version": role.Version, "updated_at": role.UpdatedAt,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRoleVersionConflict
		}
		if err := replaceRolePermissions(ctx, tx, role); err != nil {
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

func (r *DBControlPlaneRepository) ListRoleTemplates(ctx context.Context, resourceType string) ([]RoleTemplateModel, error) {
	var out []RoleTemplateModel
	query, args := whereBuilder().eq("resource_type", resourceType).build()
	if err := r.db.FindMany(ctx, &out, query, args...); err != nil {
		return nil, err
	}
	for i := range out {
		if err := loadRolePermissions(ctx, r.db, &out[i]); err != nil {
			return nil, err
		}
		count, err := r.CountActiveGrantsByRole(ctx, out[i].ID, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		out[i].ActiveGrantCount = count
	}
	return out, nil
}

func (r *DBControlPlaneRepository) CountActiveGrantsByRole(ctx context.Context, roleTemplateID string, at time.Time) (int64, error) {
	return r.db.Count(ctx, &GrantModel{}, "role_template_id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", roleTemplateID, at)
}

type rolePermissionStore interface {
	FindMany(ctx context.Context, dest any, query any, args ...any) error
}

func loadRolePermissions(ctx context.Context, store rolePermissionStore, role *RoleTemplateModel) error {
	var rows []RoleTemplatePermissionModel
	if err := store.FindMany(ctx, &rows, "role_template_id = ? ORDER BY sort_order ASC, permission ASC", role.ID); err != nil {
		return err
	}
	role.Permissions = make([]string, 0, len(rows))
	for _, row := range rows {
		role.Permissions = append(role.Permissions, row.Permission)
	}
	return nil
}

func replaceRolePermissions(ctx context.Context, tx dbx.Tx, role *RoleTemplateModel) error {
	if err := tx.Delete(ctx, &RoleTemplatePermissionModel{}, "role_template_id = ?", role.ID); err != nil {
		return err
	}
	for index, permission := range role.Permissions {
		row := &RoleTemplatePermissionModel{ID: fmt.Sprintf("%s:%s", role.ID, permission), RoleTemplateID: role.ID, Permission: permission, SortOrder: index}
		if err := tx.Create(ctx, row); err != nil {
			return err
		}
	}
	return nil
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

func (r *DBControlPlaneRepository) GetGrant(ctx context.Context, id, orgID string) (*GrantModel, error) {
		var out GrantModel
		if err := r.db.FindOne(ctx, &out, "id = ? AND org_id = ?", id, orgID); err != nil {
			return nil, err
		}
		return &out, nil
	}

func (r *DBControlPlaneRepository) RevokeGrant(ctx context.Context, id, orgID string, revokedAt time.Time, audit *GrantAuditModel, outbox ...*OutboxEventModel) error {
		return r.db.InTx(ctx, func(tx dbx.Tx) error {
			if err := tx.Update(ctx, &GrantModel{}, "id = ? AND org_id = ? AND revoked_at IS NULL", []any{id, orgID}, map[string]any{"revoked_at": revokedAt}); err != nil {
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
		wb := whereBuilder().eq("org_id", opts.OrgID).eq("resource_type", opts.ResourceType).eq("resource_id", opts.ResourceID).eq("subject_type", opts.SubjectType).eq("subject_id", opts.SubjectID).eq("relation", opts.Relation).eq("role_key", opts.RoleKey).eq("source", opts.Source)
		if opts.Active != nil {
			if *opts.Active {
				wb = wb.isNull("revoked_at")
			} else {
				wb = wb.isNotNull("revoked_at")
			}
		}
		query, args := wb.build()
		res, err := r.db.Paginate(ctx, &out, &GrantModel{}, query, args, opts.Page, opts.Size)
		return pageFrom(out, res, err)
	}

func (r *DBControlPlaneRepository) ListDueExpiringGrants(ctx context.Context, limit int) ([]GrantModel, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var out []GrantModel
	now := time.Now().UTC()
	return out, r.db.FindMany(ctx, &out, "expires_at IS NOT NULL AND expires_at <= ? AND revoked_at IS NULL ORDER BY expires_at ASC LIMIT ?", now, limit)
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

// ListOutboxEventsForRetry returns up to limit outbox events that are due for
// (re)dispatch: pending, submitted, or failed rows whose next_run_at has
// elapsed. Failed rows that have exhausted their retry budget are excluded so
// the worker does not spin on dead-lettered events.
func (r *DBControlPlaneRepository) ListOutboxEventsForRetry(ctx context.Context, limit int) ([]OutboxEventModel, error) {
	if limit <= 0 {
		limit = 50
	}
	var out []OutboxEventModel
	err := r.db.GORM(ctx).Where(
		"status IN ? AND (next_run_at IS NULL OR next_run_at <= ?) AND retry_count < ?",
		[]string{StatusPending, StatusSubmitted, StatusFailed}, time.Now().UTC(), MaxOutboxRetries,
	).Order("created_at ASC").Limit(limit).Find(&out).Error
	return out, err
}

// IncrementOutboxRetry atomically bumps retry_count by one for the event and
// returns the updated row so the caller can apply dead-lettering once the
// budget is exhausted.
func (r *DBControlPlaneRepository) IncrementOutboxRetry(ctx context.Context, id string) (*OutboxEventModel, error) {
	if err := r.db.Increment(ctx, &OutboxEventModel{}, "id = ?", []any{id}, "retry_count", 1); err != nil {
		return nil, err
	}
	return r.GetOutboxEvent(ctx, id)
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

func (w *where) isNull(col string) *where {
		w.parts = append(w.parts, col+" IS NULL")
		return w
	}

func (w *where) isNotNull(col string) *where {
		w.parts = append(w.parts, col+" IS NOT NULL")
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
