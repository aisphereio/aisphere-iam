// Package grant owns product-level RBAC operations and projects them to
// kernel/authz ReBAC relationships.
package grant

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authz"

	"github.com/aisphereio/aisphere-iam/internal/biz/graph"
	"github.com/aisphereio/aisphere-iam/internal/biz/idgen"
	"github.com/aisphereio/aisphere-iam/internal/biz/projection"
	"github.com/aisphereio/aisphere-iam/internal/data"
)

type ResourceRef struct{ Type, ID string }
type SubjectRef struct{ Type, ID, Relation string }

type RegisterRoleTemplateRequest struct {
	ID           string
	ResourceType string
	RoleKey      string
	DisplayName  string
	Description  string
	Relation     string
	BuiltIn      bool
	Enabled      bool
	SortOrder    int
	MetadataJSON string
	Permissions  []string
	Actor        SubjectRef
}

type UpdateRoleTemplateRequest struct {
	ID              string
	DisplayName     string
	Description     string
	Permissions     []string
	ExpectedVersion int64
	Actor           SubjectRef
}

type DisableRoleTemplateRequest struct {
	ID                  string
	ExpectedVersion     int64
	ConfirmActiveGrants bool
	Actor               SubjectRef
}

type RoleTemplateImpact struct {
	ActiveGrantCount   int64
	AddedPermissions   []string
	RemovedPermissions []string
}

type GrantAccessRequest struct {
	Resource  ResourceRef
	RoleKey   string
	Subject   SubjectRef
	Source    string
	Reason    string
	ExpiresAt *time.Time
	CreatedBy SubjectRef
}

type RevokeAccessRequest struct {
	GrantID                 string
	Reason                  string
	DeleteGraphRelationship bool
	Actor                   SubjectRef
}

type ExplainAccessRequest struct {
	Resource   ResourceRef
	Permission string
	Subject    SubjectRef
}

type ExplainAccessReply struct {
	Allowed          bool
	Effect           string
	Reason           string
	ConsistencyToken string
}

type Service struct {
	repo       data.ControlPlaneRepository
	authorizer authz.Authorizer
	projection *projection.Manager
	now        func() time.Time
}

func NewService(repo data.ControlPlaneRepository, authorizer authz.Authorizer, writer authz.RelationshipWriter, managers ...*projection.Manager) *Service {
	pm := firstProjectionManager(managers)
	if pm == nil {
		pm = projection.NewManager(repo, writer, nil)
	}
	return &Service{repo: repo, authorizer: authorizer, projection: pm, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) RegisterRoleTemplate(ctx context.Context, req RegisterRoleTemplateRequest) (*data.RoleTemplateModel, error) {
	if s.repo == nil {
		return nil, errors.New("grant service repository is nil")
	}
	req.ResourceType = strings.TrimSpace(req.ResourceType)
	req.RoleKey = strings.TrimSpace(req.RoleKey)
	req.Relation = strings.TrimSpace(req.Relation)
	if req.ResourceType == "" || req.RoleKey == "" {
		return nil, errors.New("resource_type and role_key are required")
	}
	permissions := normalizeStrings(req.Permissions)
	if req.Relation == "" {
		var err error
		permissions, err = s.validateCustomRolePermissions(ctx, req.ResourceType, permissions)
		if err != nil {
			return nil, err
		}
		req.Relation = "custom_binding"
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = req.ResourceType + ":" + req.RoleKey
	}
	now := s.now()
	role := &data.RoleTemplateModel{
		ID:           id,
		ResourceType: req.ResourceType,
		RoleKey:      req.RoleKey,
		DisplayName:  nonEmpty(req.DisplayName, req.RoleKey),
		Description:  strings.TrimSpace(req.Description),
		Relation:     req.Relation,
		BuiltIn:      req.BuiltIn,
		Enabled:      req.Enabled,
		SortOrder:    req.SortOrder,
		MetadataJSON: jsonOr(req.MetadataJSON, "{}"),
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
		Permissions:  permissions,
	}
	if !req.BuiltIn && !req.Enabled {
		// The zero value for bool is false. For caller ergonomics, a custom role
		// with no explicit enabled value should still be usable. Built-ins from
		// defaults.yaml pass Enabled=true explicitly.
		role.Enabled = true
	}
	if role.Relation != "custom_binding" {
		if err := s.repo.UpsertRoleTemplate(ctx, role); err != nil {
			return nil, err
		}
		return role, nil
	}
	rels := roleCapabilityRelationships(role)
	event, err := s.projection.NewWriteEvent("role_template", role.ID, rels...)
	if err != nil {
		return nil, err
	}
	audit := roleTemplateAudit("create", nil, role, req.Actor, now)
	if err := s.repo.SaveRoleTemplate(ctx, role, audit, event); err != nil {
		return nil, err
	}
	if _, err := s.projection.Dispatch(ctx, event); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *Service) PreviewRoleTemplateImpact(ctx context.Context, id string, permissions []string) (*RoleTemplateImpact, error) {
	role, err := s.customRole(ctx, id)
	if err != nil {
		return nil, err
	}
	desired, err := s.validateCustomRolePermissions(ctx, role.ResourceType, permissions)
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CountActiveGrantsByRole(ctx, role.ID, s.now())
	if err != nil {
		return nil, err
	}
	added, removed := stringSetDiff(role.Permissions, desired)
	return &RoleTemplateImpact{ActiveGrantCount: count, AddedPermissions: added, RemovedPermissions: removed}, nil
}

func (s *Service) UpdateRoleTemplate(ctx context.Context, req UpdateRoleTemplateRequest) (*data.RoleTemplateModel, error) {
	role, err := s.customRole(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.ExpectedVersion < 1 {
		return nil, errors.New("expected_version must be at least 1")
	}
	desired, err := s.validateCustomRolePermissions(ctx, role.ResourceType, req.Permissions)
	if err != nil {
		return nil, err
	}
	before := cloneRoleTemplate(role)
	if strings.TrimSpace(req.DisplayName) != "" {
		role.DisplayName = strings.TrimSpace(req.DisplayName)
	}
	role.Description = strings.TrimSpace(req.Description)
	role.Permissions = desired
	role.UpdatedAt = s.now()
	event, err := s.projection.NewReplaceEvent("role_template", role.ID, roleCapabilityRelationships(before), roleCapabilityRelationships(role))
	if err != nil {
		return nil, err
	}
	audit := roleTemplateAudit("update", before, role, req.Actor, role.UpdatedAt)
	if err := s.repo.UpdateRoleTemplate(ctx, role, req.ExpectedVersion, audit, event); err != nil {
		if errors.Is(err, data.ErrRoleVersionConflict) {
			return nil, errors.New("role template version conflict")
		}
		return nil, err
	}
	if _, err := s.projection.Dispatch(ctx, event); err != nil {
		return nil, err
	}
	role.ActiveGrantCount, _ = s.repo.CountActiveGrantsByRole(ctx, role.ID, s.now())
	return role, nil
}

func (s *Service) DisableRoleTemplate(ctx context.Context, req DisableRoleTemplateRequest) (*data.RoleTemplateModel, error) {
	role, err := s.customRole(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CountActiveGrantsByRole(ctx, role.ID, s.now())
	if err != nil {
		return nil, err
	}
	if count > 0 && !req.ConfirmActiveGrants {
		return nil, errors.New("role template has active grants; confirmation is required")
	}
	before := cloneRoleTemplate(role)
	role.Enabled = false
	role.UpdatedAt = s.now()
	audit := roleTemplateAudit("disable", before, role, req.Actor, role.UpdatedAt)
	if err := s.repo.UpdateRoleTemplate(ctx, role, req.ExpectedVersion, audit); err != nil {
		if errors.Is(err, data.ErrRoleVersionConflict) {
			return nil, errors.New("role template version conflict")
		}
		return nil, err
	}
	role.ActiveGrantCount = count
	return role, nil
}

func normalizeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (s *Service) validateCustomRolePermissions(ctx context.Context, resourceType string, permissions []string) ([]string, error) {
	permissions = normalizeStrings(permissions)
	if len(permissions) == 0 {
		return nil, errors.New("custom role permissions are required")
	}
	model, err := s.repo.GetResourceType(ctx, strings.TrimSpace(resourceType))
	if err != nil {
		return nil, err
	}
	if !model.Grantable {
		return nil, errors.New("resource type is not grantable: " + resourceType)
	}
	var allowed []string
	if err := json.Unmarshal([]byte(model.PermissionsJSON), &allowed); err != nil {
		return nil, errors.New("resource type permissions are invalid")
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, permission := range allowed {
		allowedSet[strings.TrimSpace(permission)] = struct{}{}
	}
	for _, permission := range permissions {
		if _, ok := allowedSet[permission]; !ok {
			return nil, errors.New("permission is not supported by resource type: " + permission)
		}
	}
	return permissions, nil
}

func (s *Service) customRole(ctx context.Context, id string) (*data.RoleTemplateModel, error) {
	role, err := s.repo.GetRoleTemplate(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if role.BuiltIn || role.Relation != "custom_binding" {
		return nil, errors.New("built-in role templates are immutable")
	}
	return role, nil
}

func roleCapabilityRelationships(role *data.RoleTemplateModel) []authz.Relationship {
	if role == nil {
		return nil
	}
	objectID := role.ResourceType + ":" + role.RoleKey
	rels := make([]authz.Relationship, 0, len(role.Permissions))
	for _, permission := range normalizeStrings(role.Permissions) {
		rels = append(rels, authz.Relationship{
			Resource: authz.ObjectRef{Type: "custom_role", ID: objectID},
			Relation: permission,
			Subject:  authz.SubjectRef{Type: "user", ID: "*"},
		})
	}
	return rels
}

func roleTemplateAudit(action string, before, after *data.RoleTemplateModel, actor SubjectRef, at time.Time) *data.RoleTemplateAuditModel {
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	roleID := ""
	version := int64(0)
	if after != nil {
		roleID = after.ID
		version = after.Version
	} else if before != nil {
		roleID = before.ID
		version = before.Version
	}
	return &data.RoleTemplateAuditModel{
		ID: idgen.New("raudit"), RoleTemplateID: roleID, Version: version, Action: action,
		ActorType: strings.TrimSpace(actor.Type), ActorID: strings.TrimSpace(actor.ID),
		BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), CreatedAt: at,
	}
}

func cloneRoleTemplate(role *data.RoleTemplateModel) *data.RoleTemplateModel {
	if role == nil {
		return nil
	}
	out := *role
	out.Permissions = append([]string(nil), role.Permissions...)
	return &out
}

func stringSetDiff(current, desired []string) (added, removed []string) {
	currentSet := make(map[string]struct{}, len(current))
	desiredSet := make(map[string]struct{}, len(desired))
	for _, value := range normalizeStrings(current) {
		currentSet[value] = struct{}{}
	}
	for _, value := range normalizeStrings(desired) {
		desiredSet[value] = struct{}{}
		if _, ok := currentSet[value]; !ok {
			added = append(added, value)
		}
	}
	for _, value := range normalizeStrings(current) {
		if _, ok := desiredSet[value]; !ok {
			removed = append(removed, value)
		}
	}
	return added, removed
}

func (s *Service) GrantAccess(ctx context.Context, req GrantAccessRequest) (*data.GrantModel, authz.WriteResult, error) {
	if s.repo == nil {
		return nil, authz.WriteResult{}, errors.New("grant service repository is nil")
	}
	req.Resource.Type = strings.TrimSpace(req.Resource.Type)
	req.Resource.ID = strings.TrimSpace(req.Resource.ID)
	req.RoleKey = strings.TrimSpace(req.RoleKey)
	if req.Resource.Type == "" || req.Resource.ID == "" || req.RoleKey == "" || subjectZero(req.Subject) {
		return nil, authz.WriteResult{}, errors.New("resource, role_key and subject are required")
	}
	role, err := s.findRoleTemplate(ctx, req.Resource.Type, req.RoleKey)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if !role.Enabled {
		return nil, authz.WriteResult{}, errors.New("role template is disabled")
	}
	if _, err := s.resourceByRef(ctx, req.Resource); err != nil {
		return nil, authz.WriteResult{}, err
	}
	id := idgen.New("grant")
	now := s.now()
	grant := &data.GrantModel{
		ID:              id,
		ResourceType:    req.Resource.Type,
		ResourceID:      req.Resource.ID,
		RoleKey:         role.RoleKey,
		RoleTemplateID:  role.ID,
		Relation:        role.Relation,
		SubjectType:     strings.TrimSpace(req.Subject.Type),
		SubjectID:       strings.TrimSpace(req.Subject.ID),
		SubjectRelation: normalizedSubjectRelation(req.Subject),
		Source:          nonEmpty(req.Source, "manual"),
		Reason:          strings.TrimSpace(req.Reason),
		ExpiresAt:       req.ExpiresAt,
		CreatedByType:   strings.TrimSpace(req.CreatedBy.Type),
		CreatedByID:     strings.TrimSpace(req.CreatedBy.ID),
		CreatedAt:       now,
	}
	audit := auditForGrant(idgen.New("gaudit"), "grant", grant, req.CreatedBy, req.Reason, now)
	spiceType, err := s.spiceType(ctx, req.Resource.Type)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	rels := []authz.Relationship{{
		Resource: graph.Object(spiceType, req.Resource.ID), Relation: role.Relation,
		Subject: toAuthzSubject(req.Subject), ExpiresAt: derefTime(req.ExpiresAt),
	}}
	if role.Relation == "custom_binding" {
		rels = customGrantRelationships(grant, spiceType)
	}
	event, err := s.projection.NewWriteEvent("grant", grant.ID, rels...)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if err := s.repo.CreateGrant(ctx, grant, audit, event); err != nil {
		return nil, authz.WriteResult{}, err
	}
	wr, err := s.projection.Dispatch(ctx, event)
	return grant, wr, err
}

func (s *Service) RevokeAccess(ctx context.Context, req RevokeAccessRequest) (authz.WriteResult, error) {
	if s.repo == nil {
		return authz.WriteResult{}, errors.New("grant service repository is nil")
	}
	req.GrantID = strings.TrimSpace(req.GrantID)
	if req.GrantID == "" {
		return authz.WriteResult{}, errors.New("grant_id is required")
	}
	existing, err := s.repo.GetGrant(ctx, req.GrantID)
	if err != nil {
		return authz.WriteResult{}, err
	}
	now := s.now()
	audit := auditForGrant(idgen.New("gaudit"), "revoke", existing, req.Actor, req.Reason, now)
	spiceType, err := s.spiceType(ctx, existing.ResourceType)
	if err != nil {
		return authz.WriteResult{}, err
	}
	filter := authz.RelationshipFilter{
		ResourceType: spiceType,
		ResourceID:   existing.ResourceID,
		Relation:     existing.Relation,
		SubjectType:  existing.SubjectType,
		SubjectID:    existing.SubjectID,
		SubjectRel:   existing.SubjectRelation,
	}
	rel := authz.Relationship{
		Resource:  graph.Object(spiceType, existing.ResourceID),
		Relation:  existing.Relation,
		Subject:   graph.Subject(existing.SubjectType, existing.SubjectID, existing.SubjectRelation),
		ExpiresAt: derefTime(existing.ExpiresAt),
	}
	var event *data.OutboxEventModel
	if existing.Relation == "custom_binding" {
		rels := customGrantRelationships(existing, spiceType)
		filters := make([]authz.RelationshipFilter, 0, len(rels))
		for _, relationship := range rels {
			filters = append(filters, relationshipFilter(relationship))
		}
		event, err = s.projection.NewBatchDeleteEvent("grant", existing.ID, filters, rels...)
	} else {
		event, err = s.projection.NewDeleteEvent("grant", existing.ID, filter, rel)
	}
	if err != nil {
		return authz.WriteResult{}, err
	}
	if err := s.repo.RevokeGrant(ctx, req.GrantID, now, audit, event); err != nil {
		return authz.WriteResult{}, err
	}
	return s.projection.Dispatch(ctx, event)
}

func (s *Service) ExplainAccess(ctx context.Context, req ExplainAccessRequest) (*ExplainAccessReply, error) {
	if s.authorizer == nil {
		return nil, errors.New("authorizer is nil")
	}
	if req.Resource.Type == "" || req.Resource.ID == "" || strings.TrimSpace(req.Permission) == "" || subjectZero(req.Subject) {
		return nil, errors.New("resource, permission and subject are required")
	}
	spiceType, err := s.spiceType(ctx, req.Resource.Type)
	if err != nil {
		return nil, err
	}
	decision, err := s.authorizer.Check(ctx, authz.CheckRequest{
		Subject:    toAuthzSubject(req.Subject),
		Resource:   graph.Object(spiceType, req.Resource.ID),
		Permission: strings.TrimSpace(req.Permission),
	})
	if err != nil {
		return nil, err
	}
	return &ExplainAccessReply{Allowed: decision.Allowed, Effect: string(decision.Effect), Reason: decision.Reason, ConsistencyToken: decision.ConsistencyToken}, nil
}

func (s *Service) findRoleTemplate(ctx context.Context, resourceType, roleKey string) (*data.RoleTemplateModel, error) {
	roles, err := s.repo.ListRoleTemplates(ctx, resourceType)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role.RoleKey == roleKey && role.Enabled {
			return &role, nil
		}
	}
	return nil, errors.New("role template not found or disabled: " + resourceType + "/" + roleKey)
}

func (s *Service) spiceType(ctx context.Context, typ string) (string, error) {
	typ = strings.TrimSpace(typ)
	switch typ {
	case "organization", "project", "zone", "group":
		return typ, nil
	}
	rt, err := s.repo.GetResourceType(ctx, typ)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(rt.SpiceDBType) == "" {
		return rt.Type, nil
	}
	return rt.SpiceDBType, nil
}

func auditForGrant(id, action string, g *data.GrantModel, actor SubjectRef, reason string, at time.Time) *data.GrantAuditModel {
	return &data.GrantAuditModel{
		ID:              id,
		GrantID:         g.ID,
		Action:          action,
		ResourceType:    g.ResourceType,
		ResourceID:      g.ResourceID,
		Relation:        g.Relation,
		SubjectType:     g.SubjectType,
		SubjectID:       g.SubjectID,
		SubjectRelation: g.SubjectRelation,
		ActorType:       strings.TrimSpace(actor.Type),
		ActorID:         strings.TrimSpace(actor.ID),
		Reason:          strings.TrimSpace(reason),
		MetadataJSON:    "{}",
		CreatedAt:       at,
	}
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(fallback)
}
func jsonOr(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}
func subjectZero(s SubjectRef) bool {
	return strings.TrimSpace(s.Type) == "" || strings.TrimSpace(s.ID) == ""
}
func normalizedSubjectRelation(s SubjectRef) string {
	rel := strings.TrimSpace(s.Relation)
	if strings.TrimSpace(s.Type) == authz.SubjectTypeGroup && rel == "" {
		return "member"
	}
	return rel
}
func toAuthzSubject(s SubjectRef) authz.SubjectRef {
	return graph.Subject(s.Type, s.ID, normalizedSubjectRelation(s))
}

func customGrantRelationships(grant *data.GrantModel, spiceType string) []authz.Relationship {
	if grant == nil {
		return nil
	}
	expiresAt := derefTime(grant.ExpiresAt)
	return []authz.Relationship{
		{
			Resource: authz.ObjectRef{Type: "role_binding", ID: grant.ID}, Relation: "role",
			Subject: authz.SubjectRef{Type: "custom_role", ID: grant.ResourceType + ":" + grant.RoleKey}, ExpiresAt: expiresAt,
		},
		{
			Resource: authz.ObjectRef{Type: "role_binding", ID: grant.ID}, Relation: "grantee",
			Subject: graph.Subject(grant.SubjectType, grant.SubjectID, grant.SubjectRelation), ExpiresAt: expiresAt,
		},
		{
			Resource: graph.Object(spiceType, grant.ResourceID), Relation: "custom_binding",
			Subject: authz.SubjectRef{Type: "role_binding", ID: grant.ID}, ExpiresAt: expiresAt,
		},
	}
}

func relationshipFilter(rel authz.Relationship) authz.RelationshipFilter {
	return authz.RelationshipFilter{
		ResourceType: rel.Resource.Type, ResourceID: rel.Resource.ID, Relation: rel.Relation,
		SubjectType: rel.Subject.Type, SubjectID: rel.Subject.ID, SubjectRel: rel.Subject.Relation,
	}
}

func (s *Service) resourceByRef(ctx context.Context, ref ResourceRef) (any, error) {
	switch strings.TrimSpace(ref.Type) {
	case "organization":
		return nil, errors.New("resource type organization is removed; use zone")
	case "project":
		return s.repo.GetProject(ctx, ref.ID)
	case "zone", "group":
		if strings.TrimSpace(ref.ID) == "" {
			return nil, errors.New("resource id is required")
		}
		return ref, nil
	default:
		return s.repo.GetResource(ctx, ref.Type, ref.ID)
	}
}

// ExpireDueGrants scans for grants whose expires_at has passed and revokes
// them. It is designed to be called from a scheduled task (taskx) and is
// idempotent: already-revoked grants are skipped by the query.
func (s *Service) ExpireDueGrants(ctx context.Context) error {
	if s.repo == nil {
		return errors.New("grant service repository is nil")
	}
	grants, err := s.repo.ListDueExpiringGrants(ctx, 100)
	if err != nil {
		return err
	}
	if len(grants) == 0 {
		return nil
	}

	var lastErr error
	for _, grant := range grants {
		if err := s.expireOne(ctx, &grant); err != nil {
			lastErr = err
			continue
		}
	}
	return lastErr
}

func (s *Service) expireOne(ctx context.Context, grant *data.GrantModel) error {
	now := s.now()
	spiceType, err := s.spiceType(ctx, grant.ResourceType)
	if err != nil {
		return err
	}
	filter := authz.RelationshipFilter{
		ResourceType: spiceType,
		ResourceID:   grant.ResourceID,
		Relation:     grant.Relation,
		SubjectType:  grant.SubjectType,
		SubjectID:    grant.SubjectID,
		SubjectRel:   grant.SubjectRelation,
	}
	rel := authz.Relationship{
		Resource:  graph.Object(spiceType, grant.ResourceID),
		Relation:  grant.Relation,
		Subject:   graph.Subject(grant.SubjectType, grant.SubjectID, grant.SubjectRelation),
		ExpiresAt: derefTime(grant.ExpiresAt),
	}
	audit := &data.GrantAuditModel{
		ID:              idgen.New("gaudit"),
		GrantID:         grant.ID,
		Action:          "expire",
		ResourceType:    grant.ResourceType,
		ResourceID:      grant.ResourceID,
		Relation:        grant.Relation,
		SubjectType:     grant.SubjectType,
		SubjectID:       grant.SubjectID,
		SubjectRelation: grant.SubjectRelation,
		ActorType:       "system",
		ActorID:         "taskx/grant-expiration-reconciler",
		Reason:          "grant expired",
		MetadataJSON:    "{}",
		CreatedAt:       now,
	}
	var event *data.OutboxEventModel
	if grant.Relation == "custom_binding" {
		rels := customGrantRelationships(grant, spiceType)
		filters := make([]authz.RelationshipFilter, 0, len(rels))
		for _, relationship := range rels {
			filters = append(filters, relationshipFilter(relationship))
		}
		event, err = s.projection.NewBatchDeleteEvent("grant", grant.ID, filters, rels...)
	} else {
		event, err = s.projection.NewDeleteEvent("grant", grant.ID, filter, rel)
	}
	if err != nil {
		return err
	}
	if err := s.repo.RevokeGrant(ctx, grant.ID, now, audit, event); err != nil {
		return err
	}
	_, err = s.projection.Dispatch(ctx, event)
	return err
}

func firstProjectionManager(managers []*projection.Manager) *projection.Manager {
	for _, manager := range managers {
		if manager != nil {
			return manager
		}
	}
	return nil
}
