package server

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/aisphereio/aisphere-iam/internal/biz/idgen"
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
	r.POST("/v1/iam/orgs/{org_id}/groups/{group_id}/users/{user_id}", assignUserToGroupHandler(resources))
	r.DELETE("/v1/iam/orgs/{org_id}/groups/{group_id}/users/{user_id}", removeUserFromGroupHandler(resources))
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
			// Casdoor's group primary key is (Owner, Name); the parent is NOT
			// part of the key, so two groups in the same org can never share a
			// Name regardless of tree position.  Generate a unique, slug-safe
			// Name and preserve the user's original input as DisplayName.
			existingNames, err := collectExistingGroupNames(ctx, resources.Identity, orgID)
			if err != nil {
				return nil, err
			}
			uniqueName := generateUniqueGroupName(payload.Name, existingNames, "")
			displayName := payload.DisplayName
			if displayName == "" {
				displayName = payload.Name // keep the user's original text visible
			}
			group, err := resources.Identity.CreateGroup(data.WithGroupOwner(ctx, principalSubject(principal)), authn.CreateGroupRequest{Group: authn.Group{
				OrgID:       orgID,
				ParentID:    payload.ParentID,
				Name:        uniqueName,
				DisplayName: displayName,
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
		// Group Name is the immutable primary key backing both Casdoor's
		// (Owner, Name) key and the SpiceDB object ID (e.g.
		// "group:aisphere/platform").  Renaming would desync authorization
		// topology, so reject any request that tries to change it.  An empty
		// name (caller omitted the field) is allowed — it means "keep current".
		if name := strings.TrimSpace(payload.Name); name != "" && !strings.EqualFold(name, groupID) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("group name is immutable and cannot be changed from %q", groupID)})
		}

		result, err := runWithGatewayPrincipal(c, func(ctx context.Context, principal authn.Principal) (any, error) {
			if err := requireAuthz(ctx, resources.Authz, principal, groupObject(orgID, groupID), "manage"); err != nil {
				return nil, err
			}
			displayName := payload.DisplayName
			if displayName == "" {
				displayName = payload.Name
			}
			group, err := resources.Identity.UpdateGroup(ctx, authn.UpdateGroupRequest{Group: authn.Group{
				ID:          groupID,
				OrgID:       orgID,
				ParentID:    payload.ParentID,
				Name:        groupID,
				DisplayName: displayName,
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

// groupSlugAllowedRe matches characters that are safe for a Casdoor group Name.
// Casdoor's Name column is effectively [a-zA-Z0-9_]; we also allow hyphens
// and collapse everything else to a hyphen for readability.
var groupSlugAllowedRe = regexp.MustCompile(`[^a-z0-9-]`)

// groupSlugCollapseRe matches runs of consecutive hyphens produced by slugification.
var groupSlugCollapseRe = regexp.MustCompile(`-{2,}`)

// slugifyGroupName converts a user-supplied name into a safe, lowercase slug
// suitable for Casdoor's group Name primary key.  Non-ASCII input (e.g.
// Chinese) collapses to hyphens and, when nothing readable remains, falls
// back to "group-<random>" so the caller still gets a valid, unique key.
// The result is capped at 80 characters to leave room for de-duplication
// suffixes (-2, -3, ...) added by generateUniqueGroupName.
func slugifyGroupName(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = groupSlugAllowedRe.ReplaceAllString(s, "-")
	s = groupSlugCollapseRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		// Non-ASCII input (or symbols-only) produced an empty slug.
		// Fall back to a short random identifier.
		return "group-" + idgen.New("")
	}
	return s
}

// generateUniqueGroupName produces a slug from rawName that is unique within
// the org (zone) by appending -2, -3, ... when the base slug already exists.
// existingNames should contain the lowercased Name of every group already in
// the org.  When skipName is non-empty it is excluded from the collision check
// (used by the update path so a group can "keep" its own name).
func generateUniqueGroupName(raw string, existingNames map[string]bool, skipName string) string {
	base := slugifyGroupName(raw)
	if !existingNames[strings.ToLower(base)] || strings.EqualFold(base, skipName) {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if len(candidate) > 100 {
			// Truncate base to keep total within Casdoor varchar(100).
			trim := 100 - len(fmt.Sprintf("-%d", i))
			if trim < 1 {
				break
			}
			candidate = fmt.Sprintf("%s-%d", base[:trim], i)
		}
		if !existingNames[strings.ToLower(candidate)] || strings.EqualFold(candidate, skipName) {
			return candidate
		}
	}
	return base + "-" + idgen.New("")
}

// collectExistingGroupNames queries the identity provider for all groups in
// orgID and returns their lowercased Name values as a set.  This is used for
// de-duplication before CreateGroup/UpdateGroup so we never rely on Casdoor's
// unique-constraint error as the primary collision guard.
func collectExistingGroupNames(ctx context.Context, identity authn.IdentityAdmin, orgID string) (map[string]bool, error) {
	groups, err := identity.ListGroups(ctx, authn.GroupFilter{OrgID: orgID})
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(groups))
	for _, g := range groups {
		if name := strings.TrimSpace(g.Name); name != "" {
			names[strings.ToLower(name)] = true
		}
	}
	return names, nil
}

func assignUserToGroupHandler(resources *data.Resources) khttp.HandlerFunc {
	return func(c khttp.Context) error {
		orgID := strings.TrimSpace(c.Vars().Get("org_id"))
		groupID := strings.TrimSpace(c.Vars().Get("group_id"))
		userID := strings.TrimSpace(c.Vars().Get("user_id"))
		if orgID == "" || groupID == "" || userID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id, group_id and user_id are required"})
		}
		_, err := runWithGatewayPrincipal(c, func(ctx context.Context, principal authn.Principal) (any, error) {
			if err := requireAuthz(ctx, resources.Authz, principal, groupObject(orgID, groupID), "manage"); err != nil {
				return nil, err
			}
			return nil, resources.Identity.AssignUserToGroup(ctx, authn.AssignUserToGroupRequest{
				OrgID:   orgID,
				GroupID: groupID,
				UserID:  userID,
			})
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func removeUserFromGroupHandler(resources *data.Resources) khttp.HandlerFunc {
	return func(c khttp.Context) error {
		orgID := strings.TrimSpace(c.Vars().Get("org_id"))
		groupID := strings.TrimSpace(c.Vars().Get("group_id"))
		userID := strings.TrimSpace(c.Vars().Get("user_id"))
		if orgID == "" || groupID == "" || userID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id, group_id and user_id are required"})
		}
		_, err := runWithGatewayPrincipal(c, func(ctx context.Context, principal authn.Principal) (any, error) {
			if err := requireAuthz(ctx, resources.Authz, principal, groupObject(orgID, groupID), "manage"); err != nil {
				return nil, err
			}
			return nil, resources.Identity.RemoveUserFromGroup(ctx, authn.AssignUserToGroupRequest{
				OrgID:   orgID,
				GroupID: groupID,
				UserID:  userID,
			})
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}
