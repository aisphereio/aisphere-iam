package server

import (
	"encoding/json"
	"net/http"
	"net/url"
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

func NewHTTPServer(cfg conf.ServerConfig, logCfg logx.Config, metricsCfg conf.MetricsConfig, logger logx.Logger, metrics metricsx.Manager, resources *data.Resources, projections *projection.Manager, authSvc *service.IAMAuthService, dirSvc *service.IAMDirectoryService, permSvc *service.IAMPermissionService, authzAdminSvc *service.IAMAuthorizationAdminService, projectSvc *service.ProjectService, resourceSvc *service.ResourceService, grantSvc *service.GrantService, securityCfg conf.SecurityConfig) *khttp.Server {
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
	if err := serverx.RegisterHTTPServices(srv, IAMBindings(resources, authSvc, dirSvc, permSvc, projectSvc, resourceSvc, grantSvc)...); err != nil {
		panic(err)
	}
	v1.RegisterIAMAuthServiceExternalAuthorizeHTTPServer(srv, authSvc)
	v1.RegisterIAMAuthorizationAdminServiceHTTPServer(srv, authzAdminSvc)
	registerIdentityGroupRoutes(srv, resources)
	registerProjectionBranches(srv, projections)

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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
