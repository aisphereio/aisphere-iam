package data

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("iam data: not found")

type MemoryControlPlaneRepository struct {
	mu               sync.RWMutex
	projects         map[string]*ProjectModel
	caps             map[string]*CapabilityModel
	projectCaps      map[string]*ProjectCapabilityModel
	resourceTypes    map[string]*ResourceTypeModel
	resources        map[string]*ResourceModel
	bindings         map[string]*ResourceBindingModel
	externalBindings map[string]*ExternalResourceBindingModel
	roles            map[string]*RoleTemplateModel
	roleAudits       map[string]*RoleTemplateAuditModel
	grants           map[string]*GrantModel
	audits           map[string]*GrantAuditModel
	events           map[string]*OutboxEventModel
	localUsers       map[string]*LocalUserModel
}

func NewMemoryControlPlaneRepository() *MemoryControlPlaneRepository {
	return &MemoryControlPlaneRepository{
		projects: map[string]*ProjectModel{}, caps: map[string]*CapabilityModel{}, projectCaps: map[string]*ProjectCapabilityModel{},
		resourceTypes: map[string]*ResourceTypeModel{}, resources: map[string]*ResourceModel{}, bindings: map[string]*ResourceBindingModel{}, externalBindings: map[string]*ExternalResourceBindingModel{},
		roles: map[string]*RoleTemplateModel{}, roleAudits: map[string]*RoleTemplateAuditModel{}, grants: map[string]*GrantModel{}, audits: map[string]*GrantAuditModel{}, events: map[string]*OutboxEventModel{}, localUsers: map[string]*LocalUserModel{},
	}
}

func nowIfZero(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}
func clone[T any](in *T) *T {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
func key(parts ...string) string { return strings.Join(parts, ":") }
func containsFold(s, q string) bool {
	if strings.TrimSpace(q) == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(q))
}
func statusOK(status, want string) bool { return want == "" || status == want }

func pageOf[T any](items []T, page, size int) *Page[T] {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = len(items)
		if size == 0 {
			size = 100
		}
	}
	if size > 100 {
		size = 100
	}
	start := (page - 1) * size
	if start > len(items) {
		start = len(items)
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return &Page[T]{Items: append([]T(nil), items[start:end]...), Total: int64(len(items)), Page: page, Size: size, HasMore: end < len(items)}
}

func (r *MemoryControlPlaneRepository) saveEvent(event *OutboxEventModel) {
	if event != nil {
		e := clone(event)
		if e.Status == "" {
			e.Status = StatusPending
		}
		r.events[e.ID] = e
	}
}

func (r *MemoryControlPlaneRepository) CreateProject(ctx context.Context, p *ProjectModel, events ...*OutboxEventModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(p)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	r.projects[v.ID] = v
	for _, event := range events {
		r.saveEvent(event)
	}
	return nil
}
func (r *MemoryControlPlaneRepository) GetProject(ctx context.Context, id string) (*ProjectModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := r.projects[id]
	if v == nil {
		return nil, fmt.Errorf("%w: project %s", ErrNotFound, id)
	}
	return clone(v), nil
}
func (r *MemoryControlPlaneRepository) ListProjects(ctx context.Context, opts ListOptions) (*Page[ProjectModel], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ProjectModel
	for _, v := range r.projects {
		if (opts.OrgID == "" || v.OrgID == opts.OrgID) && statusOK(v.Status, opts.Status) && containsFold(v.Slug+" "+v.DisplayName, opts.Q) {
			out = append(out, *clone(v))
		}
	}
	return pageOf(out, opts.Page, opts.Size), nil
}

func (r *MemoryControlPlaneRepository) ArchiveProject(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.projects[id]
	if v == nil {
		return fmt.Errorf("%w: project %s", ErrNotFound, id)
	}
	v.Status = StatusArchived
	v.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *MemoryControlPlaneRepository) UpsertCapability(ctx context.Context, cap *CapabilityModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(cap)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	r.caps[v.ID] = v
	return nil
}
func (r *MemoryControlPlaneRepository) ListCapabilities(ctx context.Context, opts ListOptions) ([]CapabilityModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []CapabilityModel
	for _, v := range r.caps {
		if statusOK(v.Status, opts.Status) {
			out = append(out, *clone(v))
		}
	}
	return out, nil
}
func (r *MemoryControlPlaneRepository) SetProjectCapability(ctx context.Context, pc *ProjectCapabilityModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(pc)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	r.projectCaps[key(v.ProjectID, v.CapabilityID)] = v
	return nil
}
func (r *MemoryControlPlaneRepository) ListProjectCapabilities(ctx context.Context, projectID string) ([]ProjectCapabilityModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ProjectCapabilityModel
	for _, v := range r.projectCaps {
		if projectID == "" || v.ProjectID == projectID {
			out = append(out, *clone(v))
		}
	}
	return out, nil
}

func (r *MemoryControlPlaneRepository) UpsertResourceType(ctx context.Context, rt *ResourceTypeModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(rt)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	if v.Status == "" {
		v.Status = StatusActive
	}
	r.resourceTypes[v.Type] = v
	return nil
}
func (r *MemoryControlPlaneRepository) GetResourceType(ctx context.Context, typ string) (*ResourceTypeModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := r.resourceTypes[typ]
	if v == nil {
		return nil, fmt.Errorf("%w: resource type %s", ErrNotFound, typ)
	}
	return clone(v), nil
}
func (r *MemoryControlPlaneRepository) ListResourceTypes(ctx context.Context, opts ListOptions) ([]ResourceTypeModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ResourceTypeModel
	for _, v := range r.resourceTypes {
		if (opts.CapabilityID == "" || v.CapabilityID == opts.CapabilityID) && statusOK(v.Status, opts.Status) {
			out = append(out, *clone(v))
		}
	}
	return out, nil
}

func (r *MemoryControlPlaneRepository) UpsertResource(ctx context.Context, res *ResourceModel, events ...*OutboxEventModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(res)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	if v.Status == "" {
		v.Status = StatusActive
	}
	r.resources[key(v.Type, v.ID)] = v
	for _, event := range events {
		r.saveEvent(event)
	}
	return nil
}
func (r *MemoryControlPlaneRepository) GetResource(ctx context.Context, typ, id string) (*ResourceModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := r.resources[key(typ, id)]
	if v == nil {
		return nil, fmt.Errorf("%w: resource %s/%s", ErrNotFound, typ, id)
	}
	return clone(v), nil
}
func (r *MemoryControlPlaneRepository) ListResources(ctx context.Context, opts ListOptions) (*Page[ResourceModel], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ResourceModel
	for _, v := range r.resources {
		if (opts.Type == "" || v.Type == opts.Type) && (opts.OrgID == "" || v.OrgID == opts.OrgID) && (opts.ProjectID == "" || v.ProjectID == opts.ProjectID) && statusOK(v.Status, opts.Status) {
			out = append(out, *clone(v))
		}
	}
	return pageOf(out, opts.Page, opts.Size), nil
}
func (r *MemoryControlPlaneRepository) ArchiveResource(ctx context.Context, typ, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.resources[key(typ, id)]
	if v == nil {
		return fmt.Errorf("%w: resource %s/%s", ErrNotFound, typ, id)
	}
	v.Status = StatusArchived
	v.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *MemoryControlPlaneRepository) DeleteResource(ctx context.Context, typ, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.resources[key(typ, id)]
	if v == nil {
		return fmt.Errorf("%w: resource %s/%s", ErrNotFound, typ, id)
	}
	v.Status = StatusDeleted
	v.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *MemoryControlPlaneRepository) BindResource(ctx context.Context, b *ResourceBindingModel, events ...*OutboxEventModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(b)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	if v.Status == "" {
		v.Status = StatusActive
	}
	r.bindings[v.ID] = v
	for _, event := range events {
		r.saveEvent(event)
	}
	return nil
}
func (r *MemoryControlPlaneRepository) ListResourceBindings(ctx context.Context, opts ListOptions) ([]ResourceBindingModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ResourceBindingModel
	for _, v := range r.bindings {
		if (opts.ResourceType == "" || v.SourceType == opts.ResourceType) && (opts.ResourceID == "" || v.SourceID == opts.ResourceID) && statusOK(v.Status, opts.Status) {
			out = append(out, *clone(v))
		}
	}
	return out, nil
}
func (r *MemoryControlPlaneRepository) UnbindResource(ctx context.Context, bindingID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.bindings, bindingID)
	return nil
}
func (r *MemoryControlPlaneRepository) ListExternalResourceBindings(ctx context.Context, opts ListOptions) ([]ExternalResourceBindingModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ExternalResourceBindingModel
	for _, v := range r.externalBindings {
		if (opts.ResourceType == "" || v.ResourceType == opts.ResourceType) && (opts.ResourceID == "" || v.ResourceID == opts.ResourceID) && statusOK(v.SyncStatus, opts.Status) {
			out = append(out, *clone(v))
		}
	}
	return out, nil
}
func (r *MemoryControlPlaneRepository) BindExternalResource(ctx context.Context, b *ExternalResourceBindingModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(b)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	if v.SyncStatus == "" {
		v.SyncStatus = StatusPending
	}
	r.externalBindings[v.ID] = v
	return nil
}

func (r *MemoryControlPlaneRepository) UpsertRoleTemplate(ctx context.Context, role *RoleTemplateModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(role)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	if v.Version == 0 {
		v.Version = 1
	}
	v.Permissions = append([]string(nil), role.Permissions...)
	r.roles[key(v.ResourceType, v.RoleKey)] = v
	return nil
}

func (r *MemoryControlPlaneRepository) SaveRoleTemplate(ctx context.Context, role *RoleTemplateModel, audit *RoleTemplateAuditModel, events ...*OutboxEventModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(role)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	if v.Version == 0 {
		v.Version = 1
	}
	v.Permissions = append([]string(nil), role.Permissions...)
	r.roles[key(v.ResourceType, v.RoleKey)] = v
	if audit != nil {
		r.roleAudits[audit.ID] = clone(audit)
	}
	for _, event := range events {
		r.saveEvent(event)
	}
	return nil
}

func (r *MemoryControlPlaneRepository) GetRoleTemplate(ctx context.Context, id string) (*RoleTemplateModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, role := range r.roles {
		if role.ID == id {
			out := clone(role)
			out.Permissions = append([]string(nil), role.Permissions...)
			for _, grant := range r.grants {
				if grant.RoleTemplateID == id && grant.RevokedAt == nil && (grant.ExpiresAt == nil || grant.ExpiresAt.After(time.Now().UTC())) {
					out.ActiveGrantCount++
				}
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("%w: role template %s", ErrNotFound, id)
}

func (r *MemoryControlPlaneRepository) UpdateRoleTemplate(ctx context.Context, role *RoleTemplateModel, expectedVersion int64, audit *RoleTemplateAuditModel, events ...*OutboxEventModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored := r.roles[key(role.ResourceType, role.RoleKey)]
	if stored == nil || stored.ID != role.ID {
		return fmt.Errorf("%w: role template %s", ErrNotFound, role.ID)
	}
	if stored.Version != expectedVersion {
		return ErrRoleVersionConflict
	}
	v := clone(role)
	v.Version = expectedVersion + 1
	v.Permissions = append([]string(nil), role.Permissions...)
	r.roles[key(v.ResourceType, v.RoleKey)] = v
	role.Version = v.Version
	if audit != nil {
		r.roleAudits[audit.ID] = clone(audit)
	}
	for _, event := range events {
		r.saveEvent(event)
	}
	return nil
}

func (r *MemoryControlPlaneRepository) CountActiveGrantsByRole(ctx context.Context, roleTemplateID string, at time.Time) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int64
	for _, grant := range r.grants {
		if grant.RoleTemplateID == roleTemplateID && grant.RevokedAt == nil && (grant.ExpiresAt == nil || grant.ExpiresAt.After(at)) {
			count++
		}
	}
	return count, nil
}
func (r *MemoryControlPlaneRepository) ListRoleTemplates(ctx context.Context, resourceType string) ([]RoleTemplateModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []RoleTemplateModel
	for _, v := range r.roles {
		if resourceType == "" || v.ResourceType == resourceType {
			item := clone(v)
			item.Permissions = append([]string(nil), v.Permissions...)
			for _, grant := range r.grants {
				if grant.RoleTemplateID == v.ID && grant.RevokedAt == nil && (grant.ExpiresAt == nil || grant.ExpiresAt.After(time.Now().UTC())) {
					item.ActiveGrantCount++
				}
			}
			out = append(out, *item)
		}
	}
	return out, nil
}

func (r *MemoryControlPlaneRepository) CreateGrant(ctx context.Context, grant *GrantModel, audit *GrantAuditModel, events ...*OutboxEventModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	g := clone(grant)
	g.CreatedAt = nowIfZero(g.CreatedAt)
	r.grants[g.ID] = g
	if audit != nil {
		r.audits[audit.ID] = clone(audit)
	}
	for _, event := range events {
		r.saveEvent(event)
	}
	return nil
}
func (r *MemoryControlPlaneRepository) GetGrant(ctx context.Context, id string) (*GrantModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := r.grants[id]
	if v == nil {
		return nil, fmt.Errorf("%w: grant %s", ErrNotFound, id)
	}
	return clone(v), nil
}
func (r *MemoryControlPlaneRepository) RevokeGrant(ctx context.Context, id string, revokedAt time.Time, audit *GrantAuditModel, events ...*OutboxEventModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.grants[id]
	if v == nil {
		return fmt.Errorf("%w: grant %s", ErrNotFound, id)
	}
	t := revokedAt
	v.RevokedAt = &t
	if audit != nil {
		r.audits[audit.ID] = clone(audit)
	}
	for _, event := range events {
		r.saveEvent(event)
	}
	return nil
}
func (r *MemoryControlPlaneRepository) ListGrants(ctx context.Context, opts ListOptions) (*Page[GrantModel], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []GrantModel
	for _, v := range r.grants {
		if v.RevokedAt != nil {
			continue
		}
		if (opts.ResourceType == "" || v.ResourceType == opts.ResourceType) && (opts.ResourceID == "" || v.ResourceID == opts.ResourceID) && (opts.SubjectType == "" || v.SubjectType == opts.SubjectType) && (opts.SubjectID == "" || v.SubjectID == opts.SubjectID) {
			out = append(out, *clone(v))
		}
	}
	return pageOf(out, opts.Page, opts.Size), nil
}

func (r *MemoryControlPlaneRepository) ListDueExpiringGrants(ctx context.Context, limit int) ([]GrantModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now().UTC()
	var out []GrantModel
	for _, v := range r.grants {
		if v.RevokedAt != nil || v.ExpiresAt == nil {
			continue
		}
		if v.ExpiresAt.Before(now) || v.ExpiresAt.Equal(now) {
			out = append(out, *clone(v))
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (r *MemoryControlPlaneRepository) GetOutboxEvent(ctx context.Context, id string) (*OutboxEventModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := r.events[id]
	if v == nil {
		return nil, fmt.Errorf("%w: outbox %s", ErrNotFound, id)
	}
	return clone(v), nil
}
func (r *MemoryControlPlaneRepository) UpdateOutboxEvent(ctx context.Context, id string, columns map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.events[id]
	if v == nil {
		return fmt.Errorf("%w: outbox %s", ErrNotFound, id)
	}
	applyColumns(v, columns)
	v.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *MemoryControlPlaneRepository) ListOutboxEventsForRetry(ctx context.Context, limit int) ([]OutboxEventModel, error) {
	if limit <= 0 {
		limit = 50
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now().UTC()
	out := make([]OutboxEventModel, 0, limit)
	for _, v := range r.events {
		if v == nil {
			continue
		}
		if v.RetryCount >= MaxOutboxRetries {
			continue
		}
		switch v.Status {
		case StatusPending, StatusSubmitted, StatusFailed:
		default:
			continue
		}
		if v.NextRunAt != nil && v.NextRunAt.After(now) {
			continue
		}
		out = append(out, *clone(v))
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *MemoryControlPlaneRepository) IncrementOutboxRetry(ctx context.Context, id string) (*OutboxEventModel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.events[id]
	if v == nil {
		return nil, fmt.Errorf("%w: outbox %s", ErrNotFound, id)
	}
	v.RetryCount++
	v.UpdatedAt = time.Now().UTC()
	return clone(v), nil
}

func applyColumns(ptr any, columns map[string]any) {
	if ptr == nil {
		return
	}
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}
	elem := rv.Elem()
	for k, v := range columns {
		name := columnToField(k)
		f := elem.FieldByName(name)
		if !f.IsValid() || !f.CanSet() {
			continue
		}
		if v == nil {
			f.Set(reflect.Zero(f.Type()))
			continue
		}
		val := reflect.ValueOf(v)
		if val.Type().AssignableTo(f.Type()) {
			f.Set(val)
		} else if val.Type().ConvertibleTo(f.Type()) {
			f.Set(val.Convert(f.Type()))
		}
	}
}
func columnToField(s string) string {
	var b strings.Builder
	upper := true
	for _, r := range s {
		if r == '_' {
			upper = true
			continue
		}
		if upper {
			b.WriteString(strings.ToUpper(string(r)))
			upper = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (r *MemoryControlPlaneRepository) ListUsers(ctx context.Context) ([]LocalUserModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []LocalUserModel
	for _, v := range r.localUsers {
		out = append(out, *clone(v))
	}
	return out, nil
}
func (r *MemoryControlPlaneRepository) SaveUser(ctx context.Context, u *LocalUserModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := clone(u)
	v.CreatedAt = nowIfZero(v.CreatedAt)
	v.UpdatedAt = nowIfZero(v.UpdatedAt)
	r.localUsers[v.Username] = v
	return nil
}
func (r *MemoryControlPlaneRepository) DeleteUser(ctx context.Context, username string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.localUsers, username)
	return nil
}
