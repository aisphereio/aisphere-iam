package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authn"
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
	r.POST("/v1/iam/orgs/{org_id}/groups", createIdentityGroupHandler(resources.Identity))
	r.PATCH("/v1/iam/orgs/{org_id}/groups/{group_id}", updateIdentityGroupHandler(resources.Identity))
	r.DELETE("/v1/iam/orgs/{org_id}/groups/{group_id}", deleteIdentityGroupHandler(resources.Identity))
}

func createIdentityGroupHandler(identity authn.IdentityAdmin) khttp.HandlerFunc {
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

		result, err := runWithGatewayPrincipal(c, func(ctx context.Context) (any, error) {
			group, err := identity.CreateGroup(ctx, authn.CreateGroupRequest{Group: authn.Group{
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
			return groupToReply(group), nil
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, result)
	}
}

func updateIdentityGroupHandler(identity authn.IdentityAdmin) khttp.HandlerFunc {
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

		result, err := runWithGatewayPrincipal(c, func(ctx context.Context) (any, error) {
			name := payload.Name
			if name == "" {
				name = groupID
			}
			group, err := identity.UpdateGroup(ctx, authn.UpdateGroupRequest{Group: authn.Group{
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
			return groupToReply(group), nil
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, result)
	}
}

func deleteIdentityGroupHandler(identity authn.IdentityAdmin) khttp.HandlerFunc {
	return func(c khttp.Context) error {
		orgID := strings.TrimSpace(c.Vars().Get("org_id"))
		groupID := strings.TrimSpace(c.Vars().Get("group_id"))
		if orgID == "" || groupID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id and group_id are required"})
		}
		recursive := strings.EqualFold(c.Query().Get("recursive"), "true")
		_, err := runWithGatewayPrincipal(c, func(ctx context.Context) (any, error) {
			return map[string]bool{"success": true}, identity.DeleteGroup(ctx, authn.DeleteGroupRequest{
				OrgID:     orgID,
				GroupID:   groupID,
				Recursive: recursive,
			})
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]bool{"success": true})
	}
}

func runWithGatewayPrincipal(c khttp.Context, fn func(context.Context) (any, error)) (any, error) {
	return c.Middleware(func(ctx context.Context, _ any) (any, error) {
		principal, ok := authn.PrincipalFromContext(ctx)
		if !ok || !principal.IsAuthenticated() {
			return nil, authn.ErrMissingCredential("gateway principal is required")
		}
		return fn(ctx)
	})(c, nil)
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
