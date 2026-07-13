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
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/logx"
)

func getTestConfigPath() string {
	if p := os.Getenv("IAM_TEST_CONFIG"); p != "" {
		return p
	}
	return "../../configs/config.test.yaml"
}

func TestIntegrationConfigLoads(t *testing.T) {
	cfg, err := conf.LoadConfig(getTestConfigPath())
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
	cfg, err := conf.LoadConfig(getTestConfigPath())
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
	cfg, err := conf.LoadConfig(getTestConfigPath())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	logger := logx.DefaultLogger()
	resources, cleanup, err := data.NewResources(context.Background(), cfg, data.ResourceOptions{Logger: logger})
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
	cfg, err := conf.LoadConfig(getTestConfigPath())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	logger := logx.DefaultLogger()
	resources, cleanup, err := data.NewResources(context.Background(), cfg, data.ResourceOptions{Logger: logger})
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

func TestIntegrationProjectLifecycle(t *testing.T) {
	cfg, err := conf.LoadConfig(getTestConfigPath())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	logger := logx.DefaultLogger()
	resources, cleanup, err := data.NewResources(context.Background(), cfg, data.ResourceOptions{Logger: logger})
	if err != nil {
		t.Fatalf("NewResources: %v", err)
	}
	defer cleanup()

	// Create a project through the full stack
	projectID := "test-integration-project"
	project := &data.ProjectModel{
		ID:          projectID,
		OrgID:       "aisphere",
		Slug:        "test-integration",
		DisplayName: "Test Integration Project",
		Status:      data.StatusActive,
	}
	if err := resources.ControlPlane.CreateProject(context.Background(), project); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Read it back
	got, err := resources.ControlPlane.GetProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Slug != "test-integration" {
		t.Fatalf("unexpected slug: %s", got.Slug)
	}

	// Archive it
	if err := resources.ControlPlane.ArchiveProject(context.Background(), projectID); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}

	t.Log("Project lifecycle integration test OK")
}

func TestIntegrationAuthnPrincipal(t *testing.T) {
	// Test that we can create a principal from Kernel context
	principal := authn.Principal{
		SubjectID:   "test-user",
		SubjectType: authn.SubjectTypeUser,
		OrgID:       "aisphere",
		Provider:    "casdoor",
	}
	ctx := authn.ContextWithPrincipal(context.Background(), principal)

	// Verify principal extraction
	got, ok := authn.PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("expected principal from context")
	}
	if got.SubjectID != "test-user" {
		t.Fatalf("unexpected subject: %s", got.SubjectID)
	}
	t.Log("Principal context test OK")
}