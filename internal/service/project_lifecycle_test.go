package service

import (
	"context"
	"testing"

	projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
	projectbiz "github.com/aisphereio/aisphere-iam/internal/biz/project"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestProjectServiceUpdateMaskAndZoneScope(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	biz := projectbiz.NewService(repo, authz.NewMemoryRelationshipStore())
	service := NewProjectService(biz, repo)
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{SubjectID: "alice", SubjectType: "user", OrgID: "zone-a"})

	created, err := service.CreateProject(ctx, &projectv1.CreateProjectRequest{Slug: "alpha", DisplayName: "Alpha"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	metadata, _ := structpb.NewStruct(map[string]any{"tier": "prod"})
	updated, err := service.UpdateProject(ctx, &projectv1.UpdateProjectRequest{
		ProjectId: created.GetId(), Description: "", Metadata: metadata,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"description", "metadata"}},
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if updated.GetDescription() != "" || updated.GetMetadata().GetFields()["tier"].GetStringValue() != "prod" {
		t.Fatalf("unexpected update response: %#v", updated)
	}
	if updated.GetCreatedBy().GetId() != "alice" {
		t.Fatalf("created_by was not returned: %#v", updated.GetCreatedBy())
	}

	other := authn.ContextWithPrincipal(context.Background(), authn.Principal{SubjectID: "bob", SubjectType: "user", OrgID: "zone-b"})
	if _, err := service.GetProject(other, &projectv1.GetProjectRequest{ProjectId: created.GetId()}); err == nil {
		t.Fatal("expected cross-zone project read to fail")
	}
	if _, err := service.UpdateProject(ctx, &projectv1.UpdateProjectRequest{ProjectId: created.GetId(), UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"slug"}}}); err == nil {
		t.Fatal("expected immutable/unknown update path to fail")
	}
}

// TestProjectServiceGetProjectNotFound verifies that a missing project is
// returned as gRPC NOT_FOUND (not a bare sentinel error), so downstream
// callers like Hub can translate it to HTTP 404 instead of 503.
func TestProjectServiceGetProjectNotFound(t *testing.T) {
	repo := data.NewMemoryControlPlaneRepository()
	biz := projectbiz.NewService(repo, authz.NewMemoryRelationshipStore())
	service := NewProjectService(biz, repo)
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{SubjectID: "alice", SubjectType: "user", OrgID: "zone-a"})

	_, err := service.GetProject(ctx, &projectv1.GetProjectRequest{ProjectId: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for missing project, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("expected codes.NotFound, got %v: %v", st.Code(), st.Message())
	}
}
