package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/gorilla/mux"
)

// LocalUserHandler provides simple CRUD handlers for local users.
// These are registered as plain HTTP handlers (not protobuf/gRPC) to match
// the legacy /v1/users API expected by the IAM frontend.
type LocalUserHandler struct {
	repo data.LocalUserRepository
}

func NewLocalUserHandler(repo data.LocalUserRepository) *LocalUserHandler {
	return &LocalUserHandler{repo: repo}
}

type localUserResponse struct {
	Username     string   `json:"username"`
	Password     string   `json:"password,omitempty"`
	SubjectID    string   `json:"subjectId"`
	SubjectType  string   `json:"subjectType"`
	DisplayName  string   `json:"displayName"`
	Email        string   `json:"email"`
	Organization string   `json:"organization"`
	Roles        []string `json:"roles"`
	Permissions  []string `json:"permissions"`
	Namespaces   []string `json:"namespaces"`
	Disabled     bool     `json:"disabled"`
}

type localUserListResponse struct {
	Users []localUserResponse `json:"users"`
}

type localUserDeleteResponse struct {
	Success bool `json:"success"`
}

func (h *LocalUserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListUsers(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := localUserListResponse{Users: make([]localUserResponse, 0, len(users))}
	for _, u := range users {
		resp.Users = append(resp.Users, modelToResponse(u))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *LocalUserHandler) SaveUser(w http.ResponseWriter, r *http.Request) {
	var req localUserResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		writeJSONError(w, http.StatusBadRequest, "username is required")
		return
	}

	model := data.LocalUserModel{
		Username:        strings.TrimSpace(req.Username),
		SubjectID:       strings.TrimSpace(req.SubjectID),
		SubjectType:     firstNonEmpty(strings.TrimSpace(req.SubjectType), "human"),
		DisplayName:     strings.TrimSpace(req.DisplayName),
		Email:           strings.TrimSpace(req.Email),
		Organization:    strings.TrimSpace(req.Organization),
		RolesJSON:       toJSON(req.Roles),
		PermissionsJSON: toJSON(req.Permissions),
		NamespacesJSON:  toJSON(req.Namespaces),
		Disabled:        req.Disabled,
	}
	if pw := strings.TrimSpace(req.Password); pw != "" {
		hash := sha256.Sum256([]byte(pw))
		model.PasswordHash = hex.EncodeToString(hash[:])
	}

	if err := h.repo.SaveUser(r.Context(), &model); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, modelToResponse(model))
}

func (h *LocalUserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := strings.TrimSpace(vars["username"])
	if username == "" {
		writeJSONError(w, http.StatusBadRequest, "username is required")
		return
	}
	if err := h.repo.DeleteUser(r.Context(), username); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, localUserDeleteResponse{Success: true})
}

func modelToResponse(u data.LocalUserModel) localUserResponse {
	return localUserResponse{
		Username:     u.Username,
		SubjectID:    u.SubjectID,
		SubjectType:  u.SubjectType,
		DisplayName:  u.DisplayName,
		Email:        u.Email,
		Organization: u.Organization,
		Roles:        fromJSON(u.RolesJSON),
		Permissions:  fromJSON(u.PermissionsJSON),
		Namespaces:   fromJSON(u.NamespacesJSON),
		Disabled:     u.Disabled,
	}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func fromJSON(s string) []string {
	var out []string
	if s == "" || s == "[]" || s == "null" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
