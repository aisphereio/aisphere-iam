package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"

	khttp "github.com/aisphereio/kernel/transportx/http"
)

const defaultOpenAPIDir = "docs/openapi"

func registerOpenAPIContracts(srv *khttp.Server) {
	if srv == nil {
		return
	}

	root := os.Getenv("AISPHERE_OPENAPI_DIR")
	if root == "" {
		root = defaultOpenAPIDir
	}

	routes := map[string]string{
		"/openapi/iam/full.swagger.json":    "iam.full.swagger.json",
		"/openapi/iam/console.swagger.json": "iam.console.swagger.json",
		"/openapi/iam/contract.json":        "openapi.lock.json",
	}
	for route, name := range routes {
		filePath := filepath.Join(root, name)
		srv.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			content, err := os.ReadFile(filePath)
			if err != nil {
				http.Error(w, "OpenAPI contract unavailable", http.StatusServiceUnavailable)
				return
			}
			digest := sha256.Sum256(content)
			etag := `"` + hex.EncodeToString(digest[:]) + `"`
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("ETag", etag)
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			_, _ = w.Write(content)
		})
	}
}
