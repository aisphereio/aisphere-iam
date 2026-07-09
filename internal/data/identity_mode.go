package data

import (
	"context"
	"fmt"
	"strings"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/dtmx"
)

const (
	IdentityModeCasdoorLocal = "casdoor_local"
	IdentityModeExternalOIDC = "external_oidc"

	identityAuthZProjectionTopic       = "iam.identity.authz.projection"
	identityAuthZProjectionOperationUp = "write"
	identityAuthZProjectionOperationRm = "delete"
)

func identityMode(mode string) (string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "", IdentityModeCasdoorLocal:
		return IdentityModeCasdoorLocal, nil
	case IdentityModeExternalOIDC:
		return IdentityModeExternalOIDC, nil
	default:
		return "", fmt.Errorf("unsupported authn identity_mode: %s", mode)
	}
}

func identityForMode(mode string, next authn.IdentityAdmin) (authn.IdentityAdmin, error) {
	resolved, err := identityMode(mode)
	if err != nil {
		return nil, err
	}
	if next == nil {
		return nil, nil
	}
	if resolved == IdentityModeExternalOIDC {
		return externalOIDCIdentityAdmin{next: next}, nil
	}
	return next, nil
}

// IdentityAuthZProjectionPayload is the DTM branch payload used to project
// Casdoor directory mutations into the SpiceDB/AuthZ relationship graph.
//
// DTM only transports the branch request. Casdoor remains the identity truth
// source; SpiceDB remains a derived authorization projection. If DTM is disabled,
// the same payload is applied inline so local development keeps working.
type IdentityAuthZProjectionPayload struct {
	Operation     string                     `json:"operation"`
	Relationships []authz.Relationship       `json:"relationships,omitempty"`
	Filters       []authz.RelationshipFilter `json:"filters,omitempty"`
}

// BindIdentityAuthZ projects identity-provider group changes into the AuthZ
// relationship graph. It is intentionally independent from identity_mode:
// casdoor_local and external_oidc both use Casdoor groups as the directory-side
// hierarchy for Aisphere authorization.
func BindIdentityAuthZ(next authn.IdentityAdmin, relationships authz.RelationshipWriter, managers ...dtmx.Manager) authn.IdentityAdmin {
	if next == nil || relationships == nil {
		return next
	}
	return authzProjectingIdentityAdmin{IdentityAdmin: next, relationships: relationships, dtm: firstDTMManager(managers)}
}

// externalOIDCIdentityAdmin protects the upstream user/org directory while still
// allowing Aisphere-owned application groups and group membership to be managed.
//
// In external OIDC mode, users and identity organizations come from an upstream
// identity source and are therefore read-only from IAM's perspective. Groups are
// different: IAM uses Casdoor groups as an application-layer authorization
// construct, including multi-level groups and user-to-group binding for local
// access projection. Those group operations intentionally remain writable.
type externalOIDCIdentityAdmin struct {
	next authn.IdentityAdmin
}

func (a externalOIDCIdentityAdmin) ExchangeCode(ctx context.Context, req authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	return a.next.ExchangeCode(ctx, req)
}

func (a externalOIDCIdentityAdmin) RefreshToken(ctx context.Context, req authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return a.next.RefreshToken(ctx, req)
}

func (a externalOIDCIdentityAdmin) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	return a.next.VerifyToken(ctx, req)
}

func (a externalOIDCIdentityAdmin) RevokeToken(ctx context.Context, req authn.RevokeTokenRequest) error {
	return a.next.RevokeToken(ctx, req)
}

func (a externalOIDCIdentityAdmin) GetUser(ctx context.Context, orgID, userID string) (authn.User, error) {
	return a.next.GetUser(ctx, orgID, userID)
}

func (a externalOIDCIdentityAdmin) FindUsers(ctx context.Context, filter authn.UserFilter) ([]authn.User, error) {
	return a.next.FindUsers(ctx, filter)
}

func (a externalOIDCIdentityAdmin) CreateUser(ctx context.Context, req authn.CreateUserRequest) (authn.User, error) {
	return authn.User{}, externalDirectoryReadOnlyError("CreateUser")
}

func (a externalOIDCIdentityAdmin) UpdateUser(ctx context.Context, req authn.UpdateUserRequest) (authn.User, error) {
	return authn.User{}, externalDirectoryReadOnlyError("UpdateUser")
}

func (a externalOIDCIdentityAdmin) DeleteUser(ctx context.Context, req authn.DeleteUserRequest) error {
	return externalDirectoryReadOnlyError("DeleteUser")
}

func (a externalOIDCIdentityAdmin) UpsertUser(ctx context.Context, user authn.User) (authn.User, error) {
	return authn.User{}, externalDirectoryReadOnlyError("UpsertUser")
}

func (a externalOIDCIdentityAdmin) DisableUser(ctx context.Context, orgID, userID string) error {
	return externalDirectoryReadOnlyError("DisableUser")
}

func (a externalOIDCIdentityAdmin) CreateOrganization(ctx context.Context, req authn.CreateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, externalDirectoryReadOnlyError("CreateOrganization")
}

func (a externalOIDCIdentityAdmin) GetOrganization(ctx context.Context, orgID string) (authn.Organization, error) {
	return a.next.GetOrganization(ctx, orgID)
}

func (a externalOIDCIdentityAdmin) UpdateOrganization(ctx context.Context, req authn.UpdateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, externalDirectoryReadOnlyError("UpdateOrganization")
}

func (a externalOIDCIdentityAdmin) DeleteOrganization(ctx context.Context, req authn.DeleteOrganizationRequest) error {
	return externalDirectoryReadOnlyError("DeleteOrganization")
}

func (a externalOIDCIdentityAdmin) CreateApplication(ctx context.Context, req authn.CreateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, externalDirectoryReadOnlyError("CreateApplication")
}

func (a externalOIDCIdentityAdmin) GetApplication(ctx context.Context, orgID, appID string) (authn.Application, error) {
	return a.next.GetApplication(ctx, orgID, appID)
}

func (a externalOIDCIdentityAdmin) UpdateApplication(ctx context.Context, req authn.UpdateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, externalDirectoryReadOnlyError("UpdateApplication")
}

func (a externalOIDCIdentityAdmin) DeleteApplication(ctx context.Context, req authn.DeleteApplicationRequest) error {
	return externalDirectoryReadOnlyError("DeleteApplication")
}

func (a externalOIDCIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	return a.next.CreateGroup(ctx, req)
}

func (a externalOIDCIdentityAdmin) GetGroup(ctx context.Context, orgID, groupID string) (authn.Group, error) {
	return a.next.GetGroup(ctx, orgID, groupID)
}

func (a externalOIDCIdentityAdmin) ListGroups(ctx context.Context, filter authn.GroupFilter) ([]authn.Group, error) {
	return a.next.ListGroups(ctx, filter)
}

func (a externalOIDCIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	return a.next.UpdateGroup(ctx, req)
}

func (a externalOIDCIdentityAdmin) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	return a.next.DeleteGroup(ctx, req)
}

func (a externalOIDCIdentityAdmin) AssignUserToGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return a.next.AssignUserToGroup(ctx, req)
}

func (a externalOIDCIdentityAdmin) RemoveUserFromGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return a.next.RemoveUserFromGroup(ctx, req)
}

func externalDirectoryReadOnlyError(operation string) error {
	return authn.ErrIdentityBackendFailed("identity user/org directory is read-only in external_oidc mode: "+operation, nil)
}

type authzProjectingIdentityAdmin struct {
	authn.IdentityAdmin
	relationships authz.RelationshipWriter
	dtm           dtmx.Manager
}

func (a authzProjectingIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	group, err := a.IdentityAdmin.CreateGroup(ctx, req)
	if err != nil {
		return authn.Group{}, err
	}
	if err := a.projectWrite(ctx, "identity.group.create", firstNonEmpty(group.ID, req.Group.ID), groupTopologyRelationships(group, req.Group)...); err != nil {
		return authn.Group{}, err
	}
	return group, nil
}

func (a authzProjectingIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	var oldGroup authn.Group
	if req.Group.OrgID != "" && req.Group.ID != "" {
		oldGroup, _ = a.IdentityAdmin.GetGroup(ctx, req.Group.OrgID, req.Group.ID)
	}
	group, err := a.IdentityAdmin.UpdateGroup(ctx, req)
	if err != nil {
		return authn.Group{}, err
	}
	filters := groupTopologyDeleteFilters(oldGroup, req.Group)
	rels := groupTopologyRelationships(group, req.Group)
	if err := a.projectDelete(ctx, "identity.group.update.cleanup", firstNonEmpty(group.ID, req.Group.ID), filters, nil); err != nil {
		return authn.Group{}, err
	}
	if err := a.projectWrite(ctx, "identity.group.update", firstNonEmpty(group.ID, req.Group.ID), rels...); err != nil {
		return authn.Group{}, err
	}
	return group, nil
}

func (a authzProjectingIdentityAdmin) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	if err := a.IdentityAdmin.DeleteGroup(ctx, req); err != nil {
		return err
	}
	return a.projectDelete(ctx, "identity.group.delete", req.GroupID, groupDeleteFilters(req.GroupID), nil)
}

func (a authzProjectingIdentityAdmin) AssignUserToGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	if err := a.IdentityAdmin.AssignUserToGroup(ctx, req); err != nil {
		return err
	}
	rels := []authz.Relationship{groupMemberRelationship(req.GroupID, req.UserID)}
	if req.OrgID != "" {
		rels = append(rels, authz.Relationship{Resource: authz.ObjectRef{Type: "zone", ID: req.OrgID}, Relation: "member", Subject: authz.SubjectRef{Type: "user", ID: req.UserID}})
	}
	return a.projectWrite(ctx, "identity.group.member.assign", req.GroupID, rels...)
}

func (a authzProjectingIdentityAdmin) RemoveUserFromGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	if err := a.IdentityAdmin.RemoveUserFromGroup(ctx, req); err != nil {
		return err
	}
	return a.projectDelete(ctx, "identity.group.member.remove", req.GroupID, []authz.RelationshipFilter{{ResourceType: "group", ResourceID: req.GroupID, Relation: "member", SubjectType: "user", SubjectID: req.UserID}}, []authz.Relationship{groupMemberRelationship(req.GroupID, req.UserID)})
}

func (a authzProjectingIdentityAdmin) projectWrite(ctx context.Context, name, aggregateID string, rels ...authz.Relationship) error {
	clean := make([]authz.Relationship, 0, len(rels))
	for _, rel := range rels {
		if rel.Resource.IsZero() || rel.Relation == "" || rel.Subject.IsZero() {
			continue
		}
		clean = append(clean, rel)
	}
	if len(clean) == 0 {
		return nil
	}
	return a.dispatchIdentityAuthZ(ctx, name, aggregateID, IdentityAuthZProjectionPayload{Operation: identityAuthZProjectionOperationUp, Relationships: clean})
}

func (a authzProjectingIdentityAdmin) projectDelete(ctx context.Context, name, aggregateID string, filters []authz.RelationshipFilter, rels []authz.Relationship) error {
	clean := make([]authz.RelationshipFilter, 0, len(filters))
	for _, filter := range filters {
		if filter.ResourceType == "" && filter.ResourceID == "" && filter.Relation == "" && filter.SubjectType == "" && filter.SubjectID == "" && filter.SubjectRel == "" {
			continue
		}
		clean = append(clean, filter)
	}
	if len(clean) == 0 {
		return nil
	}
	return a.dispatchIdentityAuthZ(ctx, name, aggregateID, IdentityAuthZProjectionPayload{Operation: identityAuthZProjectionOperationRm, Filters: clean, Relationships: rels})
}

func (a authzProjectingIdentityAdmin) dispatchIdentityAuthZ(ctx context.Context, name, aggregateID string, payload IdentityAuthZProjectionPayload) error {
	if a.dtm != nil && a.dtm.Enabled() {
		gid, err := a.dtm.NewGID(ctx)
		if err != nil {
			return err
		}
		saga := dtmx.NewSaga(gid, firstNonEmpty(name, identityAuthZProjectionTopic)).
			AddHTTP("identity-authz", a.dtm.BranchURL("iam/identity-authz/apply"), a.dtm.BranchURL("iam/identity-authz/compensate"), payload)
		_, err = a.dtm.SubmitSaga(ctx, saga)
		return err
	}
	_, err := ApplyIdentityAuthZProjection(ctx, a.relationships, payload)
	return err
}

func ApplyIdentityAuthZProjection(ctx context.Context, writer authz.RelationshipWriter, payload IdentityAuthZProjectionPayload) (authz.WriteResult, error) {
	if writer == nil {
		return authz.WriteResult{}, authz.ErrBackendFailed("authz relationship writer is not configured", nil)
	}
	switch payload.Operation {
	case identityAuthZProjectionOperationUp:
		return writer.WriteRelationships(ctx, payload.Relationships...)
	case identityAuthZProjectionOperationRm:
		var out authz.WriteResult
		for _, filter := range payload.Filters {
			part, err := writer.DeleteRelationships(ctx, filter)
			out.Deleted += part.Deleted
			if part.ConsistencyToken != "" {
				out.ConsistencyToken = part.ConsistencyToken
			}
			if err != nil {
				return out, err
			}
		}
		return out, nil
	default:
		return authz.WriteResult{}, fmt.Errorf("unsupported identity authz projection operation: %s", payload.Operation)
	}
}

func CompensateIdentityAuthZProjection(ctx context.Context, writer authz.RelationshipWriter, payload IdentityAuthZProjectionPayload) (authz.WriteResult, error) {
	if writer == nil {
		return authz.WriteResult{}, authz.ErrBackendFailed("authz relationship writer is not configured", nil)
	}
	switch payload.Operation {
	case identityAuthZProjectionOperationUp:
		var out authz.WriteResult
		for _, rel := range payload.Relationships {
			part, err := writer.DeleteRelationships(ctx, authz.RelationshipFilter{ResourceType: rel.Resource.Type, ResourceID: rel.Resource.ID, Relation: rel.Relation, SubjectType: rel.Subject.Type, SubjectID: rel.Subject.ID, SubjectRel: rel.Subject.Relation})
			out.Deleted += part.Deleted
			if part.ConsistencyToken != "" {
				out.ConsistencyToken = part.ConsistencyToken
			}
			if err != nil {
				return out, err
			}
		}
		return out, nil
	case identityAuthZProjectionOperationRm:
		return writer.WriteRelationships(ctx, payload.Relationships...)
	default:
		return authz.WriteResult{}, fmt.Errorf("unsupported identity authz projection operation: %s", payload.Operation)
	}
}

func groupTopologyRelationships(primary authn.Group, fallback authn.Group) []authz.Relationship {
	groupID := firstNonEmpty(primary.ID, fallback.ID)
	orgID := firstNonEmpty(primary.OrgID, fallback.OrgID)
	parentID := firstNonEmpty(primary.ParentID, fallback.ParentID)
	if groupID == "" {
		return nil
	}
	rels := make([]authz.Relationship, 0, 3)
	if orgID != "" {
		rels = append(rels, authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: groupID}, Relation: "zone", Subject: authz.SubjectRef{Type: "zone", ID: orgID}})
	}
	if parentID != "" && parentID != groupID && parentID != orgID {
		rels = append(rels,
			authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: groupID}, Relation: "parent", Subject: authz.SubjectRef{Type: "group", ID: parentID}},
			authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: parentID}, Relation: "member", Subject: authz.SubjectRef{Type: "group", ID: groupID, Relation: "member"}},
		)
	}
	return rels
}

func groupTopologyDeleteFilters(oldGroup authn.Group, fallback authn.Group) []authz.RelationshipFilter {
	groupID := firstNonEmpty(oldGroup.ID, fallback.ID)
	if groupID == "" {
		return nil
	}
	return []authz.RelationshipFilter{
		{ResourceType: "group", ResourceID: groupID, Relation: "zone"},
		{ResourceType: "group", ResourceID: groupID, Relation: "parent"},
		{ResourceType: "group", Relation: "member", SubjectType: "group", SubjectID: groupID, SubjectRel: "member"},
	}
}

func groupDeleteFilters(groupID string) []authz.RelationshipFilter {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil
	}
	return []authz.RelationshipFilter{
		{ResourceType: "group", ResourceID: groupID},
		{SubjectType: "group", SubjectID: groupID},
		{SubjectType: "group", SubjectID: groupID, SubjectRel: "member"},
	}
}

func groupMemberRelationship(groupID, userID string) authz.Relationship {
	return authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: strings.TrimSpace(groupID)}, Relation: "member", Subject: authz.SubjectRef{Type: "user", ID: strings.TrimSpace(userID)}}
}

func firstDTMManager(managers []dtmx.Manager) dtmx.Manager {
	for _, manager := range managers {
		if manager != nil {
			return manager
		}
	}
	return nil
}

var _ authn.IdentityAdmin = externalOIDCIdentityAdmin{}
var _ authn.IdentityAdmin = authzProjectingIdentityAdmin{}
