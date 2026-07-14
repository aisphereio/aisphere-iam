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

type identityGroupMembershipRequest struct {
	OrgID   string `json:"org_id"`
	GroupID string `json:"group_id"`
	UserID  string `json:"user_id"`
}

func registerIdentityMembershipRoutes(srv *khttp.Server, resources *data.Resources) {
	if srv == nil || resources == nil || resources.Identity == nil {
		return
	}
	r := srv.Route("")
	r.POST("/v1/iam/directory/group-memberships:assign", assignIdentityGroupMembershipHandler(resources))
	r.POST("/v1/iam/directory/group-memberships:remove", removeIdentityGroupMembershipHandler(resources))
}

func assignIdentityGroupMembershipHandler(resources *data.Resources) khttp.HandlerFunc {
	return handleIdentityGroupMembership(resources, true)
}

func removeIdentityGroupMembershipHandler(resources *data.Resources) khttp.HandlerFunc {
	return handleIdentityGroupMembership(resources, false)
}

func handleIdentityGroupMembership(resources *data.Resources, assign bool) khttp.HandlerFunc {
	return func(c khttp.Context) error {
		var req identityGroupMembershipRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		req.OrgID = strings.TrimSpace(req.OrgID)
		req.GroupID = strings.TrimSpace(req.GroupID)
		req.UserID = strings.TrimSpace(req.UserID)
		if req.OrgID == "" || req.GroupID == "" || req.UserID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "org_id, group_id and user_id are required"})
		}

		result, err := runWithGatewayPrincipal(c, func(ctx context.Context, principal authn.Principal) (any, error) {
			if err := requireIdentityGroupMembershipPermission(ctx, resources, principal, req.OrgID); err != nil {
				return nil, err
			}

			input := authn.AssignUserToGroupRequest{OrgID: req.OrgID, GroupID: req.GroupID, UserID: req.UserID}
			if assign {
				if err := resources.Identity.AssignUserToGroup(ctx, input); err != nil {
					return nil, err
				}
			} else {
				if err := resources.Identity.RemoveUserFromGroup(ctx, input); err != nil {
					return nil, err
				}
			}
			return map[string]any{"success": true, "assigned": assign}, nil
		})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, result)
	}
}

func requireIdentityGroupMembershipPermission(ctx context.Context, resources *data.Resources, principal authn.Principal, orgID string) error {
	if resources == nil || resources.Authz == nil {
		return authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	subjectType := strings.TrimSpace(principal.SubjectType)
	if subjectType == "" {
		subjectType = authz.SubjectTypeUser
	}
	decision, err := resources.Authz.Check(ctx, authz.CheckRequest{
		Subject:    authz.SubjectRef{Type: subjectType, ID: principal.SubjectID},
		Resource:   authz.ObjectRef{Type: "zone", ID: orgID},
		Permission: "manage_groups",
		OrgID:      orgID,
	})
	if err != nil {
		return err
	}
	if !decision.IsAllowed() {
		return authz.ErrPermissionDenied("spicedb check permission failed: zone:" + orgID + "#manage_groups@" + subjectType + ":" + principal.SubjectID)
	}
	return nil
}
