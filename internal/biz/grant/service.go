// Package grant owns product-level RBAC operations and projects them to
// kernel/authz ReBAC relationships.
package grant

import (
	"context"
	"errors"
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
	if req.ResourceType == "" || req.RoleKey == "" || req.Relation == "" {
		return nil, errors.New("resource_type, role_key and relation are required")
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
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if !req.BuiltIn && !req.Enabled {
		// The zero value for bool is false. For caller ergonomics, a custom role
		// with no explicit enabled value should still be usable. Built-ins from
		// defaults.yaml pass Enabled=true explicitly.
		role.Enabled = true
	}
	if err := s.repo.UpsertRoleTemplate(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
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
	rel := authz.Relationship{
		Resource:  graph.Object(spiceType, req.Resource.ID),
		Relation:  role.Relation,
		Subject:   toAuthzSubject(req.Subject),
		ExpiresAt: derefTime(req.ExpiresAt),
	}
	event, err := s.projection.NewWriteEvent("grant", grant.ID, rel)
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
	event, err := s.projection.NewDeleteEvent("grant", existing.ID, filter, rel)
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
	event, err := s.projection.NewDeleteEvent("grant", grant.ID, filter, rel)
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
