package service

import (
	"context"
	"testing"

	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	"github.com/aisphereio/kernel/authn"
)

func TestCurrentPrincipalSubjectUsesKernelAuthnContext(t *testing.T) {
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID:   "496333c7-7acc-4717-8596-056544fc0a68",
		SubjectType: authn.SubjectTypeUser,
		Provider:    "gateway",
		OrgID:       "aisphere",
		AuthMethod:  authn.AuthMethodOIDC,
	})

	typ, id, err := currentPrincipalSubject(ctx)
	if err != nil {
		t.Fatalf("currentPrincipalSubject returned error: %v", err)
	}
	if typ != authn.SubjectTypeUser || id != "496333c7-7acc-4717-8596-056544fc0a68" {
		t.Fatalf("unexpected subject: %s/%s", typ, id)
	}
}

func TestCurrentPrincipalSubjectRejectsMissingPrincipal(t *testing.T) {
	if _, _, err := currentPrincipalSubject(context.Background()); err == nil {
		t.Fatal("expected missing principal error")
	}
}

func TestProjectContextComesOnlyFromKernelPrincipal(t *testing.T) {
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID:   "user-1",
		SubjectType: authn.SubjectTypeUser,
		OrgID:       "casdoor-org-1",
	})

	orgID, projectActor, err := currentProjectContext(ctx)
	if err != nil {
		t.Fatalf("currentProjectContext returned error: %v", err)
	}
	if orgID != "casdoor-org-1" {
		t.Fatalf("project context used unexpected organization: %q", orgID)
	}
	if projectActor.Type != authn.SubjectTypeUser || projectActor.ID != "user-1" {
		t.Fatalf("project context used unexpected actor: %#v", projectActor)
	}
}

func TestProjectContextRejectsPrincipalWithoutOrganization(t *testing.T) {
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID:   "user-1",
		SubjectType: authn.SubjectTypeUser,
	})
	if _, _, err := currentProjectContext(ctx); err == nil {
		t.Fatal("expected missing principal org_id error")
	}
}

func TestResourceOwnerFallbackDefaultsToKernelPrincipal(t *testing.T) {
	ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{
		SubjectID:   "user-1",
		SubjectType: authn.SubjectTypeUser,
	})

	resourceActor, err := currentResourceSubject(ctx)
	if err != nil {
		t.Fatalf("currentResourceSubject returned error: %v", err)
	}
	resourceOwner := resourceSubjectOr(&resourcev1.SubjectRef{}, resourceActor)
	if resourceOwner.Type != authn.SubjectTypeUser || resourceOwner.ID != "user-1" {
		t.Fatalf("resource fallback did not use actor: %#v", resourceOwner)
	}
}
