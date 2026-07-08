package server

import (
	"encoding/json"
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
	srv.HandleFunc("/v1/iam/directory/group-memberships:assign", func(w http.ResponseWriter, r *http.Request) {
		handleIdentityGroupMembership(w, r, resources, true)
	})
	srv.HandleFunc("/v1/iam/directory/group-memberships:remove", func(w http.ResponseWriter, r *http.Request) {
		handleIdentityGroupMembership(w, r, resources, false)
	})
}

func handleIdentityGroupMembership(w http.ResponseWriter, r *http.Request, resources *data.Resources, assign bool) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if resources == nil || resources.Identity == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "identity provider is not configured"})
		return
	}

	var req identityGroupMembershipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	req.OrgID = strings.TrimSpace(req.OrgID)
	req.GroupID = strings.TrimSpace(req.GroupID)
	req.UserID = strings.TrimSpace(req.UserID)
	if req.OrgID == "" || req.GroupID == "" || req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "org_id, group_id and user_id are required"})
		return
	}

	if err := requireIdentityGroupMembershipPermission(r, resources, req.OrgID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	input := authn.AssignUserToGroupRequest{OrgID: req.OrgID, GroupID: req.GroupID, UserID: req.UserID}
	var err error
	if assign {
		err = resources.Identity.AssignUserToGroup(r.Context(), input)
	} else {
		err = resources.Identity.RemoveUserFromGroup(r.Context(), input)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "assigned": assign})
}

func requireIdentityGroupMembershipPermission(r *http.Request, resources *data.Resources, orgID string) error {
	principal, ok := authn.PrincipalFromContext(r.Context())
	if !ok || !principal.IsAuthenticated() {
		return authn.ErrMissingCredential("gateway principal is required")
	}
	if resources == nil || resources.Authz == nil {
		return authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	subjectType := strings.TrimSpace(principal.SubjectType)
	if subjectType == "" {
		subjectType = authz.SubjectTypeUser
	}
	decision, err := resources.Authz.Check(r.Context(), authz.CheckRequest{
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
