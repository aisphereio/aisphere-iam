package service

import (
	"context"
	"strings"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

type IAMAuthorizationAdminService struct {
	v1.UnimplementedIAMAuthorizationAdminServiceServer
	deps IAMDeps
}

func NewIAMAuthorizationAdminService(deps IAMDeps) *IAMAuthorizationAdminService {
	return &IAMAuthorizationAdminService{deps: deps}
}

func (s *IAMAuthorizationAdminService) GetAuthorizationSchema(ctx context.Context, req *v1.GetAuthorizationSchemaRequest) (*v1.AuthorizationSchema, error) {
	if err := s.requireGlobalAuthz(ctx, "view_schema"); err != nil {
		return nil, err
	}
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	schema, err := s.deps.Authz.ReadSchema(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.AuthorizationSchema{Text: schema.Text, Version: schema.Version}, nil
}

func (s *IAMAuthorizationAdminService) ValidateAuthorizationSchema(ctx context.Context, req *v1.ValidateAuthorizationSchemaRequest) (*v1.ValidateAuthorizationSchemaReply, error) {
	if err := s.requireGlobalAuthz(ctx, "publish_schema"); err != nil {
		return nil, err
	}
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	if err := s.deps.Authz.ValidateSchema(ctx, authz.Schema{Text: req.GetText()}); err != nil {
		return &v1.ValidateAuthorizationSchemaReply{Valid: false, Error: err.Error()}, nil
	}
	return &v1.ValidateAuthorizationSchemaReply{Valid: true}, nil
}

func (s *IAMAuthorizationAdminService) PublishAuthorizationSchema(ctx context.Context, req *v1.PublishAuthorizationSchemaRequest) (*v1.PublishAuthorizationSchemaReply, error) {
	if err := s.requireGlobalAuthz(ctx, "publish_schema"); err != nil {
		return nil, err
	}
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	if strings.TrimSpace(req.GetText()) == "" {
		return nil, authn.ErrInvalidTokenRequest("schema text is required")
	}
	if err := s.deps.Authz.WriteSchema(ctx, authz.Schema{Text: req.GetText()}); err != nil {
		return nil, err
	}
	return &v1.PublishAuthorizationSchemaReply{Published: true}, nil
}

func (s *IAMAuthorizationAdminService) ListRelationships(ctx context.Context, req *v1.ListRelationshipsRequest) (*v1.ListRelationshipsReply, error) {
	if err := s.requireGlobalAuthz(ctx, "view_relationships"); err != nil {
		return nil, err
	}
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	rels, err := s.deps.Authz.ReadRelationships(ctx, authz.RelationshipFilter{
		ResourceType: req.GetResourceType(),
		ResourceID:   req.GetResourceId(),
		Relation:     req.GetRelation(),
		SubjectType:  req.GetSubjectType(),
		SubjectID:    req.GetSubjectId(),
		SubjectRel:   req.GetSubjectRelation(),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*v1.Relationship, 0, len(rels))
	for _, rel := range rels {
		out = append(out, relationshipToProto(rel))
	}
	return &v1.ListRelationshipsReply{Relationships: out}, nil
}

func (s *IAMAuthorizationAdminService) WriteRelationships(ctx context.Context, req *v1.WriteRelationshipsRequest) (*v1.WriteRelationshipsReply, error) {
	if err := s.requireGlobalAuthz(ctx, "repair_relationships"); err != nil {
		return nil, err
	}
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	inputs := req.GetRelationships()
	if len(inputs) == 0 {
		return nil, authn.ErrInvalidTokenRequest("at least one relationship is required")
	}
	rels := make([]authz.Relationship, 0, len(inputs))
	for _, input := range inputs {
		rel := relationshipFromProto(input)
		if rel.Resource.IsZero() || strings.TrimSpace(rel.Relation) == "" || rel.Subject.IsZero() {
			return nil, authn.ErrInvalidTokenRequest("resource, relation and subject are required")
		}
		rels = append(rels, rel)
	}
	result, err := s.deps.Authz.WriteRelationships(ctx, rels...)
	if err != nil {
		return nil, err
	}
	return &v1.WriteRelationshipsReply{Written: int32(result.Written), ConsistencyToken: result.ConsistencyToken}, nil
}

func (s *IAMAuthorizationAdminService) DeleteRelationships(ctx context.Context, req *v1.DeleteRelationshipsRequest) (*v1.DeleteRelationshipsReply, error) {
	if err := s.requireGlobalAuthz(ctx, "repair_relationships"); err != nil {
		return nil, err
	}
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	result, err := s.deps.Authz.DeleteRelationships(ctx, relationshipFilterFromProto(req.GetFilter()))
	if err != nil {
		return nil, err
	}
	return &v1.DeleteRelationshipsReply{Deleted: int32(result.Deleted), ConsistencyToken: result.ConsistencyToken}, nil
}

func (s *IAMAuthorizationAdminService) CheckAuthorization(ctx context.Context, req *v1.CheckPermissionRequest) (*v1.CheckPermissionReply, error) {
	if err := s.requireGlobalAuthz(ctx, "view_relationships"); err != nil {
		return nil, err
	}
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	decision, err := s.deps.Authz.Check(ctx, authz.CheckRequest{
		Subject:    subjectFromProto(req.GetSubject()),
		Resource:   objectFromProto(req.GetResource()),
		Permission: req.GetPermission(),
		OrgID:      req.GetOrgId(),
		ProjectID:  req.GetProjectId(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.CheckPermissionReply{Allowed: decision.IsAllowed(), Effect: string(decision.Effect), Reason: decision.Reason, ConsistencyToken: decision.ConsistencyToken}, nil
}

func (s *IAMAuthorizationAdminService) ExplainAuthorization(ctx context.Context, req *v1.ExplainAuthorizationRequest) (*v1.ExplainAuthorizationReply, error) {
	if err := s.requireGlobalAuthz(ctx, "view_relationships"); err != nil {
		return nil, err
	}
	check := req.GetCheck()
	if check == nil {
		return nil, authn.ErrInvalidTokenRequest("check request is required")
	}
	decision, err := s.deps.Authz.Check(ctx, authz.CheckRequest{
		Subject:    subjectFromProto(check.GetSubject()),
		Resource:   objectFromProto(check.GetResource()),
		Permission: check.GetPermission(),
		OrgID:      check.GetOrgId(),
		ProjectID:  check.GetProjectId(),
	})
	if err != nil {
		return nil, err
	}
	steps := []string{
		"subject=" + subjectFromProto(check.GetSubject()).String(),
		"resource=" + objectFromProto(check.GetResource()).String(),
		"permission=" + strings.TrimSpace(check.GetPermission()),
		"decision=" + string(decision.Effect),
	}
	if decision.Reason != "" {
		steps = append(steps, "reason="+decision.Reason)
	}
	return &v1.ExplainAuthorizationReply{Allowed: decision.IsAllowed(), Effect: string(decision.Effect), Reason: decision.Reason, ConsistencyToken: decision.ConsistencyToken, Steps: steps}, nil
}

func (s *IAMAuthorizationAdminService) GetEffectivePermissions(ctx context.Context, req *v1.GetEffectivePermissionsRequest) (*v1.GetEffectivePermissionsReply, error) {
	if err := s.requireGlobalAuthz(ctx, "view_relationships"); err != nil {
		return nil, err
	}
	permissions := req.GetPermissions()
	if len(permissions) == 0 {
		permissions = []string{"read", "view", "manage", "edit", "delete", "view_users", "manage_users", "view_groups", "manage_groups", "view_permissions", "manage_permissions"}
	}
	out := make(map[string]*v1.PermissionDecision, len(permissions))
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		decision, err := s.deps.Authz.Check(ctx, authz.CheckRequest{Subject: subjectFromProto(req.GetSubject()), Resource: objectFromProto(req.GetResource()), Permission: permission})
		if err != nil {
			out[permission] = &v1.PermissionDecision{Allowed: false, Effect: "error", Reason: err.Error()}
			continue
		}
		out[permission] = &v1.PermissionDecision{Allowed: decision.IsAllowed(), Effect: string(decision.Effect), Reason: decision.Reason}
	}
	return &v1.GetEffectivePermissionsReply{Subject: req.GetSubject(), Resource: req.GetResource(), Permissions: out}, nil
}

func (s *IAMAuthorizationAdminService) requireGlobalAuthz(ctx context.Context, permission string) error {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || !principal.IsAuthenticated() {
		return authn.ErrMissingCredential("gateway principal is required")
	}
	if s.deps.Authz == nil {
		return authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	subjectType := principal.SubjectType
	if subjectType == "" {
		subjectType = authz.SubjectTypeUser
	}
	decision, err := s.deps.Authz.Check(ctx, authz.CheckRequest{
		Subject:    authz.SubjectRef{Type: subjectType, ID: principal.SubjectID},
		Resource:   authz.ObjectRef{Type: "iam_authz", ID: "global"},
		Permission: permission,
		OrgID:      principal.OrgID,
		ProjectID:  principal.ProjectID,
	})
	if err != nil {
		return err
	}
	if !decision.IsAllowed() {
		return authz.ErrPermissionDenied("spicedb check permission failed: iam_authz:global#" + permission + "@" + subjectType + ":" + principal.SubjectID)
	}
	return nil
}

func relationshipToProto(in authz.Relationship) *v1.Relationship {
	return &v1.Relationship{Resource: objectToProto(in.Resource), Relation: in.Relation, Subject: subjectToProto(in.Subject)}
}
