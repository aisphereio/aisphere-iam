// Integration tests against aisphere-dev (36.137.200.194)
// Run with: go test -tags=integration ./internal/service/ -run TestIntegration -v
// Requires: configs/config.test.yaml with valid remote service credentials

//go:build integration

package service

import (
	"context"
	"os"
	"testing"

	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/logx"
)

func TestIntegrationConfigLoads(t *testing.T) {
	cfg, err := conf.LoadConfig("../../configs/config.test.yaml")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Security.Authn.Provider != "casdoor" {
		t.Fatalf("expected casdoor provider, got: %s", cfg.Security.Authn.Provider)
	}
	if cfg.Data.Database.Config.DSN == "" {
		t.Fatal("expected non-empty DSN")
	}
	t.Logf("DSN: %s", cfg.Data.Database.Config.DSN)
}

func TestIntegrationPostgresConnection(t *testing.T) {
	cfg, err := conf.Load("../../configs/config.test.yaml")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	logger := logx.DefaultLogger()
	resources, cleanup, err := data.NewResources(context.Background(), cfg, data.ResourceOptions{Logger: logger})
	if err != nil {
		t.Fatalf("NewResources: %v", err)
	}
	defer cleanup()

	if resources.DB == nil {
		t.Fatal("expected non-nil DB")
	}
	if err := resources.DB.PingContext(context.Background()); err != nil {
		t.Fatalf("DB Ping: %v", err)
	}
	t.Log("PostgreSQL connection OK")
}

func TestIntegrationCasdoorConnection(t *testing.T) {
	cfg := conf.LoadConfig("../../configs/config.test.yaml")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	logger := logx.DefaultLogger()
	resources, cleanup, err := data.NewResources(cfg, data.ResourceOptions{Logger: logger})
	if err != nil {
		t.Fatalf("NewResources: %v", err)
	}
	defer cleanup()

	if resources.Authn == nil {
		t.Fatal("expected non-nil Authn")
	}
	// Try to get the admin user from Casdoor
	user, err := resources.Identity.GetUser(context.Background(), "aisphere", "admin")
	if err != nil {
		t.Fatalf("GetUser(admin): %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("expected username 'admin', got: %s", user.Username)
	}
	t.Logf("Casdoor connection OK, user: %s", user.Username)
}

func TestIntegrationSpiceDBConnection(t *testing.T) {
	cfg, err := conf.LoadConfig("../../configs/config.test.yaml")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	logger := logx.DefaultLogger()
	resources, cleanup, err := data.NewResources(cfg, data.ResourceOptions{Logger: logger})
	if err != nil {
		t.Fatalf("NewResources: %v", err)
	}
	defer cleanup()

	if resources.Authz == nil {
		t.Fatal("expected authorizer")
	}

	// Try a simple permission check
	decision, err := resources.Authz.Check(context.Background(), authz.CheckRequest{
		Subject:    authz.SubjectRef{Type: "user", ID: "admin"},
		Resource:   authz.ObjectRef{Type: "zone", ID: "aisphere"},
		Permission: "view_zone",
	})
	if err != nil {
		t.Fatalf("SpiceDB Check: %v", err)
	}
	t.Logf("SpiceDB check result: allowed=%v, effect=%s", decision.IsAllowed(), decision.Effect)
}