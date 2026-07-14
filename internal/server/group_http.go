package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	khttp "github.com/aisphereio/kernel/transportx/http"
)

type groupWritePayload struct {
	ParentID    string   `json:"parentId"`
	ParentIDAlt string   `json:"parent_id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	DisplayAlt  string   `json:"display_name"`
	Type        string   `json:"type"`
	Users       []string `json:"users"`
}

type groupReply struct {
	ID          string   `json:"id"`
	ExternalID  string   `json:"externalId,omitempty"`
	OrgID       string   `json:"orgId,omitempty"`
	ParentID    string   `json:"parentId,omitempty"`
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName,omitempty"`
	Type        string   `json:"type,omitempty"`
	Path        string   `json:"path,omitempty"`
	Users       []string `json:"users,omitempty"`
}

func registerIdentityGroupRoutes(srv *khttp.Server, resources *data.Resources) {
	if srv == nil || resources == nil || resources.Identity == nil {
		return
	}
	r := srv.Route("")
	r.POST("/v1/iam/orgs/{org_id}/groups", createIdentityGroupHandler(resources))
	r.PATCH("/v1/iam/orgs/{org_id}/groups/{group_id}", updateIdentityGroupHandler(resources))
	r.DELETE("/v1/iam/orgs/{org_id}/groups/{group_id}", deleteIdentityGroupHandler(resources))
}

func createIdentityGroupHandler(resources *data.Resources) khttp.HandlerFunc {
	return func(c khttp.Context) error {
		orgID := strings.TrimSpace(c.Vars().Get("org_id"))
		var payload groupWritePayload
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		payload = normalizeGroupPayload(payload)
		if orgID == "" || payload.Name == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id and name are required"})
		}

		result, err := runWithGatewayPrincipal(c, func(ctx context.Context, principal authn.Principal) (any, error) {
			if payload.ParentID == "" {
				if err := requireAuthz(ctx, resources.Authz, principal, authz.ObjectRef{Type: "zone", ID: orgID}, "create_groups"); err != nil {
					return nil, err
				}
			} else {
				if err := requireAuthz(ctx, resources.Authz, principal, groupObject(orgID, payload.ParentID), "create_child_groups"); err != nil {
					return nil, err
				}
			}
			group, err := resources.Identity.CreateGroup(ctx, authn.CreateGroupRequest{Group: authn.Group{
				OrgID:       orgID,
				ParentID:    payload.ParentID,
				Name:        payload.Name,
				DisplayName: payload.DisplayName,
				Type:        defaultGroupType(payload.Type),
				Users:       cleanStrings(payload.Users),
			}})
			if err != nil {
				return nil, err
			}
			if err := writeGroupStructure(ctx, resources.AuthzAdmin, orgID, group); err != nil {
				return nil, err
			}
			return groupToReply(group), nil
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, result)
	}
}

func updateIdentityGroupHandler(resources *data.Resources) khttp.HandlerFunc {
	return func(c khttp.Context) error {
		orgID := strings.TrimSpace(c.Vars().Get("org_id"))
		groupID := strings.TrimSpace(c.Vars().Get("group_id"))
		var payload groupWritePayload
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		payload = normalizeGroupPayload(payload)
		if orgID == "" || groupID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id and group_id are required"})
		}

		result, err := runWithGatewayPrincipal(c, func(ctx context.Context, principal authn.Principal) (any, error) {
			if err := requireAuthz(ctx, resources.Authz, principal, groupObject(orgID, groupID), "manage"); err != nil {
				return nil, err
			}
			name := payload.Name
			if name == "" {
				name = groupID
			}
			group, err := resources.Identity.UpdateGroup(ctx, authn.UpdateGroupRequest{Group: authn.Group{
				ID:          groupID,
				OrgID:       orgID,
				ParentID:    payload.ParentID,
				Name:        name,
				DisplayName: payload.DisplayName,
				Type:        defaultGroupType(payload.Type),
				Users:       cleanStrings(payload.Users),
			}})
			if err != nil {
				return nil, err
			}
			if err := writeGroupStructure(ctx, resources.AuthzAdmin, orgID, group); err != nil {
				return nil, err
			}
			return groupToReply(group), nil
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, result)
	}
}

func deleteIdentityGroupHandler(resources *data.Resources) khttp.HandlerFunc {
	return func(c khttp.Context) error {
		orgID := strings.TrimSpace(c.Vars().Get("org_id"))
		groupID := strings.TrimSpace(c.Vars().Get("group_id"))
		if orgID == "" || groupID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id and group_id are required"})
		}
		recursive := strings.EqualFold(c.Query().Get("recursive"), "true")
		_, err := runWithGatewayPrincipal(c, func(ctx context.Context, principal authn.Principal) (any, error) {
			if err := requireAuthz(ctx, resources.Authz, principal, groupObject(orgID, groupID), "manage"); err != nil {
				return nil, err
			}
			if err := resources.Identity.DeleteGroup(ctx, authn.DeleteGroupRequest{
				OrgID:     orgID,
				GroupID:   groupID,
				Recursive: recursive,
			}); err != nil {
				return nil, err
			}
			if resources.AuthzAdmin != nil {
				if _, err := resources.AuthzAdmin.DeleteRelationships(ctx, authz.RelationshipFilter{ResourceType: "group", ResourceID: groupResourceID(orgID, groupID)}); err != nil {
					return nil, err
				}
			}
			return map[string]bool{"success": true}, nil
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]bool{"success": true})
	}
}

func runWithGatewayPrincipal(c khttp.Context, fn func(context.Context, authn.Principal) (any, error)) (any, error) {
	return c.Middleware(func(ctx context.Context, _ any) (any, error) {
		principal, ok := authn.PrincipalFromContext(ctx)
		if !ok || !principal.IsAuthenticated() {
			return nil, authn.ErrMissingCredential("gateway principal is required")
		}
		return fn(ctx, principal.Normalize())
	})(c, nil)
}

func requireAuthz(ctx context.Context, authorizer authz.Authorizer, principal authn.Principal, resource authz.ObjectRef, permission string) error {
	if authorizer == nil {
		return authz.ErrBackendFailed("authorization provider is not configured", nil)
	}
	decision, err := authorizer.Check(ctx, authz.CheckRequest{
		Subject:    principalSubject(principal),
		Resource:   resource,
		Permission: permission,
		OrgID:      principal.OrgID,
		TenantID:   principal.TenantID,
		ProjectID:  principal.ProjectID,
	})
	if err != nil {
		return err
	}
	if !decision.IsAllowed() {
		return authz.ErrPermissionDenied("permission denied: " + resource.String() + "#" + permission)
	}
	return nil
}

func principalSubject(principal authn.Principal) authz.SubjectRef {
	subjectType := strings.TrimSpace(principal.SubjectType)
	switch subjectType {
	case authz.SubjectTypeService, "service_account":
		return authz.SubjectRef{Type: subjectType, ID: principal.SubjectID}
	default:
		return authz.SubjectRef{Type: authz.SubjectTypeUser, ID: principal.SubjectID}
	}
}

func writeGroupStructure(ctx context.Context, writer authz.RelationshipWriter, zoneID string, group authn.Group) error {
	if writer == nil {
		return nil
	}
	groupID := firstNonEmptyString(group.ID, group.Name)
	if strings.TrimSpace(zoneID) == "" || strings.TrimSpace(groupID) == "" {
		return nil
	}
	rels := []authz.Relationship{
		{
			Resource: authz.ObjectRef{Type: "group", ID: groupResourceID(zoneID, groupID)},
			Relation: "zone",
			Subject:  authz.SubjectRef{Type: "zone", ID: data.SanitizeObjectID(zoneID)},
		},
	}
	if parentID := strings.TrimSpace(group.ParentID); parentID != "" {
		rels = append(rels, authz.Relationship{
			Resource: authz.ObjectRef{Type: "group", ID: groupResourceID(zoneID, groupID)},
			Relation: "parent",
			Subject:  authz.SubjectRef{Type: "group", ID: groupResourceID(zoneID, parentID)},
		})
	}
	_, err := writer.WriteRelationships(ctx, rels...)
	return err
}

func groupObject(zoneID, groupID string) authz.ObjectRef {
	return authz.ObjectRef{Type: "group", ID: groupResourceID(zoneID, groupID)}
}

func groupResourceID(zoneID, groupID string) string {
	zoneID = strings.Trim(strings.TrimSpace(zoneID), "/")
	groupID = strings.Trim(strings.TrimSpace(groupID), "/")
	if zoneID == "" {
		return data.SanitizeObjectID(groupID)
	}
	if groupID == "" {
		return data.SanitizeObjectID(zoneID)
	}
	return data.SanitizeObjectID(zoneID) + "/" + data.SanitizeObjectID(groupID)
}

func normalizeGroupPayload(in groupWritePayload) groupWritePayload {
	in.ParentID = firstNonEmptyString(strings.TrimSpace(in.ParentID), strings.TrimSpace(in.ParentIDAlt))
	in.Name = strings.TrimSpace(in.Name)
	in.DisplayName = firstNonEmptyString(strings.TrimSpace(in.DisplayName), strings.TrimSpace(in.DisplayAlt))
	in.Type = strings.TrimSpace(in.Type)
	return in
}

func defaultGroupType(groupType string) string {
	groupType = strings.TrimSpace(groupType)
	if groupType == "" {
		return authn.GroupTypeVirtual
	}
	return groupType
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
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
	return out
}

func groupToReply(group authn.Group) groupReply {
	return groupReply{
		ID:          firstNonEmptyString(group.ID, group.Name),
		ExternalID:  group.ExternalID,
		OrgID:       group.OrgID,
		ParentID:    group.ParentID,
		Name:        group.Name,
		DisplayName: group.DisplayName,
		Type:        group.Type,
		Path:        group.Path,
		Users:       append([]string(nil), group.Users...),
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
