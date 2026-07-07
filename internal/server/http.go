package server

import (
	"encoding/json"
	"net/http"
	"time"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/aisphere-iam/internal/biz/projection"
	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/service"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/metricsx"
	"github.com/aisphereio/kernel/serverx"
	khttp "github.com/aisphereio/kernel/transportx/http"
)

const externalAuthzPath = "/internal/iam/ext-authz"

func NewHTTPServer(cfg conf.ServerConfig, logCfg logx.Config, metricsCfg conf.MetricsConfig, logger logx.Logger, metrics metricsx.Manager, resources *data.Resources, projections *projection.Manager, authSvc *service.IAMAuthService, dirSvc *service.IAMDirectoryService, permSvc *service.IAMPermissionService, projectSvc *service.ProjectService, resourceSvc *service.ResourceService, grantSvc *service.GrantService, securityCfg conf.SecurityConfig) *khttp.Server {
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
	registerExternalAuthz(srv, authSvc)
	registerProjectionBranches(srv, projections)

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

func registerExternalAuthz(srv *khttp.Server, authSvc *service.IAMAuthService) {
	if authSvc == nil {
		return
	}
	srv.HandleFunc(externalAuthzPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodOptions {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		cred, ok := authn.BearerCredential(r.Header.Get("Authorization"))
		if !ok || cred.Token == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="aisphere"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}
		if _, err := authSvc.VerifyToken(r.Context(), &v1.VerifyTokenRequest{Token: cred.Token, TokenType: "access_token"}); err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="aisphere", error="invalid_token"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer token"})
			return
		}
		// ExternalAuth is an allow/deny boundary only. Do not inject X-Aisphere-*
		// principal headers here; upstream services should verify the forwarded
		// Authorization bearer token with oidc_jwt/casdoor_jwt authn mode.
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
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
