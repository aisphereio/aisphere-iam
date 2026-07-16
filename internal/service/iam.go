package service

import (
	"context"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type IAMDeps struct {
	Tokens   authn.TokenService
	Identity authn.IdentityAdmin
	Authz    authz.AdminProvider
}

type IAMAuthService struct {
	v1.UnimplementedIAMAuthServiceServer
	deps IAMDeps
}

func NewIAMAuthService(deps IAMDeps) *IAMAuthService {
	return &IAMAuthService{deps: deps}
}

func (s *IAMAuthService) VerifyToken(ctx context.Context, req *v1.VerifyTokenRequest) (*v1.Principal, error) {
	if s.deps.Tokens == nil {
		return nil, authn.ErrIdentityBackendFailed("token provider is not configured", nil)
	}
	principal, err := s.deps.Tokens.VerifyToken(ctx, authn.VerifyTokenRequest{
		Token:     req.GetToken(),
		TokenType: req.GetTokenType(),
		OrgID:     req.GetOrgId(),
		AppID:     req.GetAppId(),
	})
	if err != nil {
		return nil, err
	}
	return principalToProto(principal), nil
}

func (s *IAMAuthService) GetMe(ctx context.Context, req *v1.GetMeRequest) (*v1.GetMeReply, error) {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || !principal.IsAuthenticated() {
		return nil, authn.ErrMissingCredential("gateway principal is required")
	}
	return &v1.GetMeReply{Principal: principalToProto(principal)}, nil
}

type IAMDirectoryService struct {
	v1.UnimplementedIAMDirectoryServiceServer
	deps IAMDeps
}

func NewIAMDirectoryService(deps IAMDeps) *IAMDirectoryService {
	return &IAMDirectoryService{deps: deps}
}

func (s *IAMDirectoryService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.User, error) {
	if err := s.requireZonePermission(ctx, req.GetOrgId(), "view_users"); err != nil {
		return nil, err
	}
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	user, err := s.deps.Identity.GetUser(ctx, req.GetOrgId(), req.GetUserId())
	if err != nil {
		return nil, err
	}
	return userToProto(user), nil
}

func (s *IAMDirectoryService) ListUsers(ctx context.Context, req *v1.ListUsersRequest) (*v1.ListUsersReply, error) {
	if err := s.requireZonePermission(ctx, req.GetOrgId(), "view_users"); err != nil {
		return nil, err
	}
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	users, err := s.deps.Identity.FindUsers(ctx, authn.UserFilter{
		OrgID:   req.GetOrgId(),
		GroupID: req.GetGroupId(),
		Role:    req.GetRole(),
		Limit:   int(req.GetPageSize()),
	})
	if err != nil {
		return nil, err
	}
	return &v1.ListUsersReply{Users: usersToProto(users)}, nil
}

func (s *IAMDirectoryService) GetOrganization(ctx context.Context, req *v1.GetOrganizationRequest) (*v1.Organization, error) {
	if err := s.requireZonePermission(ctx, req.GetOrgId(), "view_zone"); err != nil {
		return nil, err
	}
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	org, err := s.deps.Identity.GetOrganization(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}
	return organizationToProto(org), nil
}

func (s *IAMDirectoryService) ListGroups(ctx context.Context, req *v1.ListGroupsRequest) (*v1.ListGroupsReply, error) {
	if err := s.requireZonePermission(ctx, req.GetOrgId(), "view_groups"); err != nil {
		return nil, err
	}
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	groups, err := s.deps.Identity.ListGroups(ctx, authn.GroupFilter{
		OrgID:    req.GetOrgId(),
		ParentID: req.GetParentId(),
		Type:     req.GetType(),
		UserID:   req.GetUserId(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.ListGroupsReply{Groups: groupsToProto(groups)}, nil
}

func (s *IAMDirectoryService) requireZonePermission(ctx context.Context, orgID string, permission string) error {
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
		Resource:   authz.ObjectRef{Type: "zone", ID: orgID},
		Permission: permission,
		OrgID:      orgID,
	})
	if err != nil {
		return err
	}
	if !decision.IsAllowed() {
		return authz.ErrPermissionDenied("spicedb check permission failed: zone:" + orgID + "#" + permission + "@" + subjectType + ":" + principal.SubjectID)
	}
	return nil
}

type IAMPermissionService struct {
	v1.UnimplementedIAMPermissionServiceServer
	deps IAMDeps
}

func NewIAMPermissionService(deps IAMDeps) *IAMPermissionService {
	return &IAMPermissionService{deps: deps}
}

func (s *IAMPermissionService) CheckPermission(ctx context.Context, req *v1.CheckPermissionRequest) (*v1.CheckPermissionReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	decision, err := s.deps.Authz.Check(ctx, permissionCheckFromProto(req))
	if err != nil {
		return nil, err
	}
	return permissionDecisionToProto(decision), nil
}

func (s *IAMPermissionService) BatchCheckPermissions(ctx context.Context, req *v1.BatchCheckPermissionsRequest) (*v1.BatchCheckPermissionsReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	inputs := req.GetChecks()
	checks := make([]authz.CheckRequest, 0, len(inputs))
	for _, input := range inputs {
		checks = append(checks, permissionCheckFromProto(input))
	}
	result, err := s.deps.Authz.BatchCheck(ctx, authz.BatchCheckRequest{Checks: checks})
	if err != nil {
		return nil, err
	}
	decisions := make([]*v1.CheckPermissionReply, 0, len(result.Decisions))
	for _, decision := range result.Decisions {
		decisions = append(decisions, permissionDecisionToProto(decision))
	}
	return &v1.BatchCheckPermissionsReply{Decisions: decisions}, nil
}

func (s *IAMPermissionService) WriteRelationships(ctx context.Context, req *v1.WriteRelationshipsRequest) (*v1.WriteRelationshipsReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	inputs := req.GetRelationships()
	rels := make([]authz.Relationship, 0, len(inputs))
	for _, input := range inputs {
		rels = append(rels, relationshipFromProto(input))
	}
	result, err := s.deps.Authz.WriteRelationships(ctx, rels...)
	if err != nil {
		return nil, err
	}
	return &v1.WriteRelationshipsReply{Written: int32(result.Written), ConsistencyToken: result.ConsistencyToken}, nil
}

func (s *IAMPermissionService) DeleteRelationships(ctx context.Context, req *v1.DeleteRelationshipsRequest) (*v1.DeleteRelationshipsReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	result, err := s.deps.Authz.DeleteRelationships(ctx, relationshipFilterFromProto(req.GetFilter()))
	if err != nil {
		return nil, err
	}
	return &v1.DeleteRelationshipsReply{Deleted: int32(result.Deleted), ConsistencyToken: result.ConsistencyToken}, nil
}

func (s *IAMPermissionService) ReadRelationships(ctx context.Context, req *v1.ListRelationshipsRequest) (*v1.ListRelationshipsReply, error) {
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

func (s *IAMPermissionService) WriteRelationship(ctx context.Context, req *v1.WriteRelationshipRequest) (*v1.WriteRelationshipReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	result, err := s.deps.Authz.WriteRelationships(ctx, relationshipFromProto(req.GetRelationship()))
	if err != nil {
		return nil, err
	}
	return &v1.WriteRelationshipReply{Written: int32(result.Written), ConsistencyToken: result.ConsistencyToken}, nil
}

func (s *IAMPermissionService) DeleteRelationship(ctx context.Context, req *v1.DeleteRelationshipRequest) (*v1.DeleteRelationshipReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	result, err := s.deps.Authz.DeleteRelationships(ctx, relationshipFilterFromProto(req.GetFilter()))
	if err != nil {
		return nil, err
	}
	return &v1.DeleteRelationshipReply{Deleted: int32(result.Deleted), ConsistencyToken: result.ConsistencyToken}, nil
}

func (s *IAMPermissionService) LookupResources(ctx context.Context, req *v1.LookupResourcesRequest) (*v1.LookupResourcesReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	result, err := s.deps.Authz.LookupResources(ctx, authz.LookupResourcesRequest{
		Subject:      subjectFromProto(req.GetSubject()),
		ResourceType: req.GetResourceType(),
		Permission:   req.GetPermission(),
		Limit:        int(req.GetLimit()),
		Cursor:       req.GetCursor(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.LookupResourcesReply{
		Resources:        objectsToProto(result.Resources),
		NextCursor:       result.NextCursor,
		ConsistencyToken: result.ConsistencyToken,
	}, nil
}

func (s *IAMPermissionService) LookupSubjects(ctx context.Context, req *v1.LookupSubjectsRequest) (*v1.LookupSubjectsReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	result, err := s.deps.Authz.LookupSubjects(ctx, authz.LookupSubjectsRequest{
		Resource:    objectFromProto(req.GetResource()),
		Permission:  req.GetPermission(),
		SubjectType: req.GetSubjectType(),
		Limit:       int(req.GetLimit()),
		Cursor:      req.GetCursor(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.LookupSubjectsReply{
		Subjects:         subjectsToProto(result.Subjects),
		NextCursor:       result.NextCursor,
		ConsistencyToken: result.ConsistencyToken,
	}, nil
}

func permissionCheckFromProto(req *v1.CheckPermissionRequest) authz.CheckRequest {
	return authz.CheckRequest{
		Subject:    subjectFromProto(req.GetSubject()),
		Resource:   objectFromProto(req.GetResource()),
		Permission: req.GetPermission(),
		OrgID:      req.GetOrgId(),
		ProjectID:  req.GetProjectId(),
	}
}

func permissionDecisionToProto(decision authz.Decision) *v1.CheckPermissionReply {
	return &v1.CheckPermissionReply{
		Allowed:          decision.IsAllowed(),
		Effect:           string(decision.Effect),
		Reason:           decision.Reason,
		ConsistencyToken: decision.ConsistencyToken,
	}
}

func tokenSetToProto(in authn.TokenSet) *v1.TokenSet {
	return &v1.TokenSet{
		AccessToken:  in.AccessToken,
		RefreshToken: in.RefreshToken,
		IdToken:      in.IDToken,
		TokenType:    in.TokenType,
		Scope:        in.Scope,
		ExpiresAt:    timestamppb.New(in.ExpiresAt),
	}
}

func principalToProto(in authn.Principal) *v1.Principal {
	in = in.Normalize()
	return &v1.Principal{
		SubjectId:   in.SubjectID,
		SubjectType: in.SubjectType,
		Provider:    in.Provider,
		ExternalId:  in.ExternalID,
		Issuer:      in.Issuer,
		Audience:    append([]string(nil), in.Audience...),
		TenantId:    in.TenantID,
		OrgId:       in.OrgID,
		AppId:       in.AppID,
		ProjectId:   in.ProjectID,
		Username:    in.Username,
		Name:        in.Name,
		Email:       in.Email,
		Phone:       in.Phone,
		Roles:       append([]string(nil), in.Roles...),
		Groups:      append([]string(nil), in.Groups...),
		Scopes:      append([]string(nil), in.Scopes...),
		AuthMethod:  in.AuthMethod,
		IssuedAt:    timestamppb.New(in.IssuedAt),
		ExpiresAt:   timestamppb.New(in.ExpiresAt),
	}
}

func userToProto(in authn.User) *v1.User {
	return &v1.User{
		Id:          in.ID,
		ExternalId:  in.ExternalID,
		Provider:    in.Provider,
		OrgId:       in.OrgID,
		Username:    in.Username,
		DisplayName: in.DisplayName,
		Email:       in.Email,
		Phone:       in.Phone,
		Roles:       append([]string(nil), in.Roles...),
		Groups:      append([]string(nil), in.Groups...),
		Enabled:     in.Enabled,
	}
}

func usersToProto(in []authn.User) []*v1.User {
	out := make([]*v1.User, 0, len(in))
	for _, user := range in {
		out = append(out, userToProto(user))
	}
	return out
}

func organizationToProto(in authn.Organization) *v1.Organization {
	return &v1.Organization{
		Id:          in.ID,
		ExternalId:  in.ExternalID,
		Name:        in.Name,
		DisplayName: in.DisplayName,
		OwnerId:     in.OwnerID,
		ParentId:    in.ParentID,
		Tags:        append([]string(nil), in.Tags...),
		Enabled:     in.Enabled,
	}
}

func groupToProto(in authn.Group) *v1.Group {
	out := &v1.Group{
		Id:          in.ID,
		ExternalId:  in.ExternalID,
		OrgId:       in.OrgID,
		Name:        in.Name,
		DisplayName: in.DisplayName,
		Type:        in.Type,
		Path:        in.Path,
		Users:       append([]string(nil), in.Users...),
	}
	if in.ParentID != "" {
		out.ParentId = &in.ParentID
	}
	return out
}

func groupsToProto(in []authn.Group) []*v1.Group {
	out := make([]*v1.Group, 0, len(in))
	for _, group := range in {
		out = append(out, groupToProto(group))
	}
	return out
}

func applicationToProto(in authn.Application) *v1.Application {
	return &v1.Application{
		Id:             in.ID,
		ExternalId:     in.ExternalID,
		OrgId:          in.OrgID,
		Name:           in.Name,
		DisplayName:    in.DisplayName,
		ClientId:       in.ClientID,
		RedirectUris:   append([]string(nil), in.RedirectURIs...),
		GrantTypes:     append([]string(nil), in.GrantTypes...),
		Scopes:         append([]string(nil), in.Scopes...),
		Providers:      append([]string(nil), in.Providers...),
		EnablePassword: in.EnablePassword,
		EnableSignup:   in.EnableSignup,
	}
}

func objectFromProto(in *v1.ObjectRef) authz.ObjectRef {
	if in == nil {
		return authz.ObjectRef{}
	}
	return authz.ObjectRef{Type: in.GetType(), ID: in.GetId()}
}

func objectToProto(in authz.ObjectRef) *v1.ObjectRef {
	return &v1.ObjectRef{Type: in.Type, Id: in.ID}
}

func objectsToProto(in []authz.ObjectRef) []*v1.ObjectRef {
	out := make([]*v1.ObjectRef, 0, len(in))
	for _, object := range in {
		out = append(out, objectToProto(object))
	}
	return out
}

func subjectFromProto(in *v1.SubjectRef) authz.SubjectRef {
	if in == nil {
		return authz.SubjectRef{}
	}
	return authz.SubjectRef{Type: in.GetType(), ID: in.GetId(), Relation: in.GetRelation()}
}

func subjectToProto(in authz.SubjectRef) *v1.SubjectRef {
	return &v1.SubjectRef{Type: in.Type, Id: in.ID, Relation: in.Relation}
}

func subjectsToProto(in []authz.SubjectRef) []*v1.SubjectRef {
	out := make([]*v1.SubjectRef, 0, len(in))
	for _, subject := range in {
		out = append(out, subjectToProto(subject))
	}
	return out
}

func relationshipFromProto(in *v1.Relationship) authz.Relationship {
	if in == nil {
		return authz.Relationship{}
	}
	return authz.Relationship{
		Resource: objectFromProto(in.GetResource()),
		Relation: in.GetRelation(),
		Subject:  subjectFromProto(in.GetSubject()),
	}
}

func relationshipFilterFromProto(in *v1.RelationshipFilter) authz.RelationshipFilter {
	if in == nil {
		return authz.RelationshipFilter{}
	}
	return authz.RelationshipFilter{
		ResourceType: in.GetResourceType(),
		ResourceID:   in.GetResourceId(),
		Relation:     in.GetRelation(),
		SubjectType:  in.GetSubjectType(),
		SubjectID:    in.GetSubjectId(),
		SubjectRel:   in.GetSubjectRelation(),
	}
}
