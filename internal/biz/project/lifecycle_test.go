package project

import (
	"context"
	"testing"
	"time"

	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authz"
)

func TestProjectLifecyclePreservesIdentityAndBlocksArchivedMutation(t *testing.T) {
	ctx := context.Background()
	repo := data.NewMemoryControlPlaneRepository()
	service := NewService(repo, authz.NewMemoryRelationshipStore())
	fixed := time.Date(2026, 7, 13, 5, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }

	project, _, err := service.CreateProject(ctx, CreateProjectRequest{
		ID: "project-1", ZoneID: "zone-a", Slug: "alpha", DisplayName: "Alpha",
		MetadataJSON: `{"tier":"dev"}`,
		CreatedBy:    SubjectRef{Type: "user", ID: "alice"},
		Owner:        SubjectRef{Type: "user", ID: "alice"},
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	display := "Alpha Updated"
	description := ""
	visibility := "org"
	labels := `{"team":"platform"}`
	metadata := `{"tier":"prod"}`
	updated, err := service.UpdateProject(ctx, UpdateProjectRequest{
		ID: project.ID, ZoneID: "zone-a", DisplayName: &display, Description: &description,
		Visibility: &visibility, LabelsJSON: &labels, MetadataJSON: &metadata,
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if updated.OrgID != "zone-a" || updated.Slug != "alpha" || updated.CreatedBy != "user:alice" {
		t.Fatalf("immutable project identity changed: %#v", updated)
	}
	if updated.DisplayName != display || updated.Description != "" || updated.Visibility != "org" || updated.MetadataJSON != `{"tier":"prod"}` {
		t.Fatalf("project fields not updated: %#v", updated)
	}

	archived, err := service.ArchiveProject(ctx, ArchiveProjectRequest{ID: project.ID, ZoneID: "zone-a"})
	if err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	if archived.Status != data.StatusArchived {
		t.Fatalf("status = %q", archived.Status)
	}
	again, err := service.ArchiveProject(ctx, ArchiveProjectRequest{ID: project.ID, ZoneID: "zone-a"})
	if err != nil || again.Status != data.StatusArchived {
		t.Fatalf("idempotent archive = (%#v, %v)", again, err)
	}
	if _, err := service.UpdateProject(ctx, UpdateProjectRequest{ID: project.ID, ZoneID: "zone-a", DisplayName: &display}); err == nil {
		t.Fatal("expected archived project update to fail")
	}
	if _, err := service.SetProjectCapability(ctx, SetProjectCapabilityRequest{ZoneID: "zone-a", ProjectID: project.ID, CapabilityID: "skills", Enabled: true}); err == nil {
		t.Fatal("expected archived project capability mutation to fail")
	}
	if _, err := service.GetProject(ctx, project.ID, "zone-b"); err == nil {
		t.Fatal("expected cross-zone read to fail")
	}
}
