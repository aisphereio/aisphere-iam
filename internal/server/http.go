package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/aisphere-iam/internal/biz/projection"
	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/service"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/serverx"
	khttp "github.com/aisphereio/kernel/transportx/http"
)

func NewHTTPServer(cfg conf.ServerConfig, logCfg logx.Config, metricsCfg conf.MetricsConfig, logger logx.Logger, metrics metricsx.Manager, resources *data.Resources, projections *projection.Manager, authSvc *service.IAMAuthService, dirSvc *service.IAMDirectoryService, groupSvc *service.IAMGroupAdminService, permSvc *service.IAMPermissionService, authzAdminSvc *service.IAMAuthorizationAdminService, projectSvc *service.ProjectService, resourceSvc *service.ResourceService, grantSvc *service.GrantService, accessQuerySvc *service.AccessQueryService, securityCfg conf.SecurityConfig) *khttp.Server {
	addr := cfg.HTTP.Addr
	if addr == "" {
		addr = "0.0.0.0:8000"
	}
	timeout := cfg.HTTP.Timeout
	if timeout <= 0 {
		timeout = time.Second
	}
	opts := []khttp.ServerOption{
		khttp.Address(addr),
		khttp.Timeout(timeout),
		khttp.Logger(logger.Named("transport.http")),
		khttp.AccessLog(logCfg.AccessLog),
		khttp.CORS(cfg.HTTP.CORS),
	}
	if metricsCfg.Enabled {
		opts = append(opts, khttp.Metrics(metrics))
	}
	if m := iamServerMiddlewares(resources, securityCfg); len(m) > 0 {
		opts = append(opts, khttp.Middleware(m...))
	}
	srv := khttp.NewServer(opts...)
	if err := serverx.RegisterHTTPServices(srv, IAMBindings(resources, authSvc, dirSvc, groupSvc, permSvc, projectSvc, resourceSvc, grantSvc, accessQuerySvc)...); err != nil {
		panic(err)
	}
v1.RegisterIAMAuthorizationAdminServiceHTTPServer(srv, authzAdminSvc)
		registerProjectionBranches(srv, projections)
		registerIdentityAuthZBranches(srv, resources)
		registerDirectoryProjectionCompatibilityRoutes(srv, service.NewDirectoryProjectionOpsFromDeps(resources.Identity, resources.AuthzAdmin, resources.IdentityProjection))

	srv.HandleFunc("/v1/iam/ui/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, safeUILoginReturnURL(r.URL.Query().Get("return_to")), http.StatusFound)
	})
	srv.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	srv.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if resources == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	return srv
}

func safeUILoginReturnURL(raw string) string {
	const fallback = "http://localhost:3001/"
	u, err := url.Parse(raw)
	if err != nil || u == nil || !u.IsAbs() {
		return fallback
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fallback
	}
	switch u.Host {
	case "localhost:3000", "localhost:3001", "127.0.0.1:3000", "127.0.0.1:3001":
		return u.String()
	default:
		return fallback
	}
}

func registerProjectionBranches(srv *khttp.Server, projections *projection.Manager) {
	if projections == nil {
		return
	}
	srv.HandleFunc("/internal/dtm/iam/projection/apply", func(w http.ResponseWriter, r *http.Request) {
		var payload projection.BranchPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if _, err := projections.ApplyEvent(r.Context(), payload.EventID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	srv.HandleFunc("/internal/dtm/iam/projection/compensate", func(w http.ResponseWriter, r *http.Request) {
		var payload projection.BranchPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if _, err := projections.CompensateEvent(r.Context(), payload.EventID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func registerIdentityAuthZBranches(srv *khttp.Server, resources *data.Resources) {
	if resources == nil || resources.IdentityProjection == nil {
		return
	}
	srv.HandleFunc("/internal/dtm/iam/identity-authz/apply", func(w http.ResponseWriter, r *http.Request) {
		var payload data.IdentityAuthZBranchPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if _, err := resources.IdentityProjection.ApplyBranch(r.Context(), payload); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	srv.HandleFunc("/internal/dtm/iam/identity-authz/compensate", func(w http.ResponseWriter, r *http.Request) {
		var payload data.IdentityAuthZBranchPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if _, err := resources.IdentityProjection.CompensateBranch(r.Context(), payload); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

type directoryProjectionRequest struct {
	OrgID string `json:"org_id"`
	Limit int    `json:"limit"`
}

// registerDirectoryProjectionCompatibilityRoutes keeps the existing HTTP paths
// alive until api/iam/v1/iam.proto is regenerated and the generated
// IAMDirectoryProjectionService HTTP server is registered. Business logic lives
// in internal/service; this function is only a temporary HTTP shim.
func registerDirectoryProjectionCompatibilityRoutes(srv *khttp.Server, ops *service.DirectoryProjectionOps) {
	if ops == nil {
		return
	}
	srv.HandleFunc("/v1/iam/directory/projections:retry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req directoryProjectionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		result, err := ops.Retry(r.Context(), req.Limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	srv.HandleFunc("/v1/iam/directory/projections:reconcile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		orgID, ok := readProjectionOrgID(w, r)
		if !ok {
			return
		}
		result, err := ops.Reconcile(r.Context(), orgID, "reconcile")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	srv.HandleFunc("/v1/iam/directory/projections:drift", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		orgID, ok := readProjectionOrgID(w, r)
		if !ok {
			return
		}
		result, err := ops.Drift(r.Context(), orgID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	srv.HandleFunc("/v1/iam/casdoor/webhooks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
		if orgID == "" {
			if raw, ok := payload["org_id"].(string); ok {
				orgID = strings.TrimSpace(raw)
			}
		}
		if orgID == "" {
			writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "reconcile": "skipped", "reason": "org_id missing"})
			return
		}
		result, err := ops.Reconcile(r.Context(), orgID, "casdoor_webhook")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	})
}

func readProjectionOrgID(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req directoryProjectionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	orgID := strings.TrimSpace(req.OrgID)
	if orgID == "" {
		orgID = strings.TrimSpace(r.URL.Query().Get("org_id"))
	}
	if orgID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "org_id is required"})
		return "", false
	}
	return orgID, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
