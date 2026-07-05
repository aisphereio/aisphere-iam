package data

import (
	"context"
	"fmt"
	"strings"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
)

const (
	IdentityModeCasdoorLocal = "casdoor_local"
	IdentityModeExternalOIDC = "external_oidc"
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

// BindIdentityOSZ projects local application group changes into the OSZ/AuthZ
// relationship graph. It is intentionally independent from identity_mode:
// casdoor_local and external_oidc both use local application groups for
// Aisphere resource authorization.
func BindIdentityOSZ(next authn.IdentityAdmin, relationships authz.RelationshipWriter) authn.IdentityAdmin {
	if next == nil || relationships == nil {
		return next
	}
	return oszProjectingIdentityAdmin{IdentityAdmin: next, relationships: relationships}
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

type oszProjectingIdentityAdmin struct {
	authn.IdentityAdmin
	relationships authz.RelationshipWriter
}

func (a oszProjectingIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	group, err := a.IdentityAdmin.CreateGroup(ctx, req)
	if err != nil {
		return authn.Group{}, err
	}
	if err := a.writeGroupParent(ctx, firstNonEmpty(group.ParentID, req.Group.ParentID), firstNonEmpty(group.ID, req.Group.ID)); err != nil {
		return authn.Group{}, err
	}
	return group, nil
}

func (a oszProjectingIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	group, err := a.IdentityAdmin.UpdateGroup(ctx, req)
	if err != nil {
		return authn.Group{}, err
	}
	if err := a.writeGroupParent(ctx, firstNonEmpty(group.ParentID, req.Group.ParentID), firstNonEmpty(group.ID, req.Group.ID)); err != nil {
		return authn.Group{}, err
	}
	return group, nil
}

func (a oszProjectingIdentityAdmin) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	if err := a.IdentityAdmin.DeleteGroup(ctx, req); err != nil {
		return err
	}
	return a.deleteGroupEdges(ctx, req.GroupID)
}

func (a oszProjectingIdentityAdmin) AssignUserToGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	if err := a.IdentityAdmin.AssignUserToGroup(ctx, req); err != nil {
		return err
	}
	return a.writeGroupMember(ctx, req.GroupID, req.UserID)
}

func (a oszProjectingIdentityAdmin) RemoveUserFromGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	if err := a.IdentityAdmin.RemoveUserFromGroup(ctx, req); err != nil {
		return err
	}
	return a.deleteGroupMember(ctx, req.GroupID, req.UserID)
}

func (a oszProjectingIdentityAdmin) writeGroupParent(ctx context.Context, parentID, groupID string) error {
	parentID = strings.TrimSpace(parentID)
	groupID = strings.TrimSpace(groupID)
	if parentID == "" || groupID == "" || parentID == groupID {
		return nil
	}
	_, err := a.relationships.WriteRelationships(ctx, authz.Relationship{
		Resource: authz.ObjectRef{Type: "group", ID: parentID},
		Relation: "member",
		Subject:  authz.SubjectRef{Type: "group", ID: groupID, Relation: "member"},
	})
	return err
}

func (a oszProjectingIdentityAdmin) writeGroupMember(ctx context.Context, groupID, userID string) error {
	groupID = strings.TrimSpace(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return nil
	}
	_, err := a.relationships.WriteRelationships(ctx, authz.Relationship{
		Resource: authz.ObjectRef{Type: "group", ID: groupID},
		Relation: "member",
		Subject:  authz.SubjectRef{Type: "user", ID: userID},
	})
	return err
}

func (a oszProjectingIdentityAdmin) deleteGroupMember(ctx context.Context, groupID, userID string) error {
	groupID = strings.TrimSpace(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return nil
	}
	_, err := a.relationships.DeleteRelationships(ctx, authz.RelationshipFilter{
		ResourceType: "group",
		ResourceID:   groupID,
		Relation:     "member",
		SubjectType:  "user",
		SubjectID:    userID,
	})
	return err
}

func (a oszProjectingIdentityAdmin) deleteGroupEdges(ctx context.Context, groupID string) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil
	}
	if _, err := a.relationships.DeleteRelationships(ctx, authz.RelationshipFilter{ResourceType: "group", ResourceID: groupID}); err != nil {
		return err
	}
	_, err := a.relationships.DeleteRelationships(ctx, authz.RelationshipFilter{SubjectType: "group", SubjectID: groupID, SubjectRel: "member"})
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var _ authn.IdentityAdmin = externalOIDCIdentityAdmin{}
var _ authn.IdentityAdmin = oszProjectingIdentityAdmin{}
