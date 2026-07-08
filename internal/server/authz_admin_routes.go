package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	khttp "github.com/aisphereio/kernel/transportx/http"
)

type authzAdminHandler struct {
	resources *data.Resources
}

type authzSchemaRequest struct {
	Text string `json:"text"`
}

type authzSchemaReply struct {
	Text    string `json:"text"`
	Version string `json:"version,omitempty"`
}

type authzRelationshipJSON struct {
	Resource authzObjectJSON  `json:"resource"`
	Relation string           `json:"relation"`
	Subject  authzSubjectJSON `json:"subject"`
}

type authzObjectJSON struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type authzSubjectJSON struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Relation string `json:"relation,omitempty"`
}

type authzRelationshipListReply struct {
	Relationships []authzRelationshipJSON `json:"relationships"`
}

type authzRelationshipWriteRequest struct {
	Relationship  authzRelationshipJSON   `json:"relationship"`
	Relationships []authzRelationshipJSON `json:"relationships,omitempty"`
}

type authzRelationshipDeleteRequest struct {
	Filter authzRelationshipFilterJSON `json:"filter"`
}

type authzRelationshipFilterJSON struct {
	ResourceType    string `json:"resourceType,omitempty"`
	ResourceID      string `json:"resourceId,omitempty"`
	Relation        string `json:"relation,omitempty"`
	SubjectType     string `json:"subjectType,omitempty"`
	SubjectID       string `json:"subjectId,omitempty"`
	SubjectRelation string `json:"subjectRelation,omitempty"`
}

type authzCheckRequest struct {
	Subject    authzSubjectJSON `json:"subject"`
	Resource   authzObjectJSON  `json:"resource"`
	Permission string           `json:"permission"`
	OrgID      string           `json:"orgId,omitempty"`
	ProjectID  string           `json:"projectId,omitempty"`
}

type authzCheckReply struct {
	Allowed          bool     `json:"allowed"`
	Effect           string   `json:"effect,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	ConsistencyToken string   `json:"consistencyToken,omitempty"`
	Steps            []string `json:"steps,omitempty"`
}

type authzEffectivePermissionsReply struct {
	Subject     authzSubjectJSON       `json:"subject"`
	Resource    authzObjectJSON        `json:"resource"`
	Permissions map[string]authzResult `json:"permissions"`
}

type authzResult struct {
	Allowed bool   `json:"allowed"`
	Effect  string `json:"effect,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func registerAuthzAdminRoutes(srv *khttp.Server, resources *data.Resources) {
	h := &authzAdminHandler{resources: resources}
	srv.HandleFunc("/v1/iam/authz/schema", h.handleSchema)
	srv.HandleFunc("/v1/iam/authz/schema:validate", h.handleValidateSchema)
	srv.HandleFunc("/v1/iam/authz/schema:publish", h.handlePublishSchema)
	srv.HandleFunc("/v1/iam/authz/relationships", h.handleRelationships)
	srv.HandleFunc("/v1/iam/authz/relationships:delete", h.handleDeleteRelationships)
	srv.HandleFunc("/v1/iam/authz/permissions:check", h.handleCheckPermission)
	srv.HandleFunc("/v1/iam/authz/permissions:explain", h.handleExplainPermission)
	srv.HandleFunc("/v1/iam/authz/effective-permissions", h.handleEffectivePermissions)
}

func (h *authzAdminHandler) handleSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !h.ensureAuthz(w, r, "view_schema") {
		return
	}
	schema, err := h.resources.AuthzAdmin.ReadSchema(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, authzSchemaReply{Text: schema.Text, Version: schema.Version})
}

func (h *authzAdminHandler) handleValidateSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !h.ensureAuthz(w, r, "publish_schema") {
		return
	}
	var req authzSchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.resources.AuthzAdmin.ValidateSchema(r.Context(), authz.Schema{Text: req.Text}); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"valid": "false", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"valid": true})
}

func (h *authzAdminHandler) handlePublishSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !h.ensureAuthz(w, r, "publish_schema") {
		return
	}
	var req authzSchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schema text is required"})
		return
	}
	if err := h.resources.AuthzAdmin.WriteSchema(r.Context(), authz.Schema{Text: req.Text}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"published": true})
}

func (h *authzAdminHandler) handleRelationships(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listRelationships(w, r)
	case http.MethodPost:
		h.writeRelationships(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *authzAdminHandler) listRelationships(w http.ResponseWriter, r *http.Request) {
	if !h.ensureAuthz(w, r, "view_relationships") {
		return
	}
	filter := authz.RelationshipFilter{
		ResourceType: strings.TrimSpace(r.URL.Query().Get("resource_type")),
		ResourceID:   strings.TrimSpace(r.URL.Query().Get("resource_id")),
		Relation:     strings.TrimSpace(r.URL.Query().Get("relation")),
		SubjectType:  strings.TrimSpace(r.URL.Query().Get("subject_type")),
		SubjectID:    strings.TrimSpace(r.URL.Query().Get("subject_id")),
		SubjectRel:   strings.TrimSpace(r.URL.Query().Get("subject_relation")),
	}
	rels, err := h.resources.AuthzAdmin.ReadRelationships(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]authzRelationshipJSON, 0, len(rels))
	for _, rel := range rels {
		out = append(out, relationshipToJSON(rel))
	}
	writeJSON(w, http.StatusOK, authzRelationshipListReply{Relationships: out})
}

func (h *authzAdminHandler) writeRelationships(w http.ResponseWriter, r *http.Request) {
	if !h.ensureAuthz(w, r, "repair_relationships") {
		return
	}
	var req authzRelationshipWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	inputs := req.Relationships
	if len(inputs) == 0 && !isZeroRelationshipJSON(req.Relationship) {
		inputs = []authzRelationshipJSON{req.Relationship}
	}
	rels := make([]authz.Relationship, 0, len(inputs))
	for _, input := range inputs {
		rel := relationshipFromJSON(input)
		if rel.Resource.IsZero() || strings.TrimSpace(rel.Relation) == "" || rel.Subject.IsZero() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "resource, relation and subject are required"})
			return
		}
		rels = append(rels, rel)
	}
	if len(rels) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one relationship is required"})
		return
	}
	result, err := h.resources.AuthzAdmin.WriteRelationships(r.Context(), rels...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"written": result.Written, "consistencyToken": result.ConsistencyToken})
}

func (h *authzAdminHandler) handleDeleteRelationships(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !h.ensureAuthz(w, r, "repair_relationships") {
		return
	}
	var req authzRelationshipDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := h.resources.AuthzAdmin.DeleteRelationships(r.Context(), authz.RelationshipFilter{
		ResourceType: req.Filter.ResourceType,
		ResourceID:   req.Filter.ResourceID,
		Relation:     req.Filter.Relation,
		SubjectType:  req.Filter.SubjectType,
		SubjectID:    req.Filter.SubjectID,
		SubjectRel:   req.Filter.SubjectRelation,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": result.Deleted, "consistencyToken": result.ConsistencyToken})
}

func (h *authzAdminHandler) handleCheckPermission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !h.ensureAuthz(w, r, "view_relationships") {
		return
	}
	var req authzCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	decision, err := h.resources.AuthzAdmin.Check(r.Context(), authz.CheckRequest{
		Subject:    subjectFromJSON(req.Subject),
		Resource:   objectFromJSON(req.Resource),
		Permission: req.Permission,
		OrgID:      req.OrgID,
		ProjectID:  req.ProjectID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, authzCheckReply{Allowed: decision.IsAllowed(), Effect: string(decision.Effect), Reason: decision.Reason, ConsistencyToken: decision.ConsistencyToken})
}

func (h *authzAdminHandler) handleExplainPermission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !h.ensureAuthz(w, r, "view_relationships") {
		return
	}
	var req authzCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	decision, err := h.resources.AuthzAdmin.Check(r.Context(), authz.CheckRequest{
		Subject:    subjectFromJSON(req.Subject),
		Resource:   objectFromJSON(req.Resource),
		Permission: req.Permission,
		OrgID:      req.OrgID,
		ProjectID:  req.ProjectID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	steps := []string{
		"subject=" + subjectFromJSON(req.Subject).String(),
		"resource=" + objectFromJSON(req.Resource).String(),
		"permission=" + strings.TrimSpace(req.Permission),
		"decision=" + string(decision.Effect),
	}
	if decision.Reason != "" {
		steps = append(steps, "reason="+decision.Reason)
	}
	writeJSON(w, http.StatusOK, authzCheckReply{Allowed: decision.IsAllowed(), Effect: string(decision.Effect), Reason: decision.Reason, ConsistencyToken: decision.ConsistencyToken, Steps: steps})
}

func (h *authzAdminHandler) handleEffectivePermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !h.ensureAuthz(w, r, "view_relationships") {
		return
	}
	query := r.URL.Query()
	subject := authzSubjectJSON{Type: query.Get("subject_type"), ID: query.Get("subject_id"), Relation: query.Get("subject_relation")}
	resource := authzObjectJSON{Type: query.Get("resource_type"), ID: query.Get("resource_id")}
	permissions := splitCSV(query.Get("permissions"))
	if len(permissions) == 0 {
		permissions = []string{"read", "view", "manage", "edit", "delete", "view_users", "manage_users", "view_groups", "manage_groups", "view_permissions", "manage_permissions"}
	}
	out := map[string]authzResult{}
	for _, permission := range permissions {
		decision, err := h.resources.AuthzAdmin.Check(r.Context(), authz.CheckRequest{Subject: subjectFromJSON(subject), Resource: objectFromJSON(resource), Permission: permission})
		if err != nil {
			out[permission] = authzResult{Allowed: false, Effect: "error", Reason: err.Error()}
			continue
		}
		out[permission] = authzResult{Allowed: decision.IsAllowed(), Effect: string(decision.Effect), Reason: decision.Reason}
	}
	writeJSON(w, http.StatusOK, authzEffectivePermissionsReply{Subject: subject, Resource: resource, Permissions: out})
}

func (h *authzAdminHandler) ensureAuthz(w http.ResponseWriter, r *http.Request, permission string) bool {
	if h == nil || h.resources == nil || h.resources.AuthzAdmin == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "authz admin provider is not configured"})
		return false
	}
	principal, ok := authn.PrincipalFromContext(r.Context())
	if !ok || !principal.IsAuthenticated() {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "gateway principal is required"})
		return false
	}
	subjectType := principal.SubjectType
	if subjectType == "" {
		subjectType = authz.SubjectTypeUser
	}
	decision, err := h.resources.AuthzAdmin.Check(r.Context(), authz.CheckRequest{
		Subject:    authz.SubjectRef{Type: subjectType, ID: principal.SubjectID},
		Resource:   authz.ObjectRef{Type: "iam_authz", ID: "global"},
		Permission: permission,
		OrgID:      firstNonEmptyQuery(r.URL.Query().Get("org_id"), principal.OrgID),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return false
	}
	if !decision.IsAllowed() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "spicedb check permission failed: iam_authz:global#" + permission + "@" + subjectType + ":" + principal.SubjectID})
		return false
	}
	return true
}

func objectFromJSON(in authzObjectJSON) authz.ObjectRef {
	return authz.ObjectRef{Type: strings.TrimSpace(in.Type), ID: strings.TrimSpace(in.ID)}
}

func objectToJSON(in authz.ObjectRef) authzObjectJSON {
	return authzObjectJSON{Type: in.Type, ID: in.ID}
}

func subjectFromJSON(in authzSubjectJSON) authz.SubjectRef {
	return authz.SubjectRef{Type: strings.TrimSpace(in.Type), ID: strings.TrimSpace(in.ID), Relation: strings.TrimSpace(in.Relation)}
}

func subjectToJSON(in authz.SubjectRef) authzSubjectJSON {
	return authzSubjectJSON{Type: in.Type, ID: in.ID, Relation: in.Relation}
}

func relationshipFromJSON(in authzRelationshipJSON) authz.Relationship {
	return authz.Relationship{Resource: objectFromJSON(in.Resource), Relation: strings.TrimSpace(in.Relation), Subject: subjectFromJSON(in.Subject)}
}

func relationshipToJSON(in authz.Relationship) authzRelationshipJSON {
	return authzRelationshipJSON{Resource: objectToJSON(in.Resource), Relation: in.Relation, Subject: subjectToJSON(in.Subject)}
}

func isZeroRelationshipJSON(in authzRelationshipJSON) bool {
	return strings.TrimSpace(in.Resource.Type) == "" && strings.TrimSpace(in.Resource.ID) == "" && strings.TrimSpace(in.Relation) == "" && strings.TrimSpace(in.Subject.Type) == "" && strings.TrimSpace(in.Subject.ID) == ""
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func firstNonEmptyQuery(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func parseLimit(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
