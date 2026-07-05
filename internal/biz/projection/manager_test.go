package projection

import (
	"context"
	"testing"

	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authz"
)

func TestManagerApplyWriteAndCompensate(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	repo := newProjectionRepo()
	manager := NewManager(repo, store, nil)
	event, err := manager.NewWriteEvent("grant", "grant-1", authz.Relationship{
		Resource: authz.ObjectRef{Type: "skill", ID: "s1"},
		Relation: "editor",
		Subject:  authz.SubjectRef{Type: "user", ID: "u1"},
	})
	if err != nil {
		t.Fatalf("NewWriteEvent returned error: %v", err)
	}
	repo.events[event.ID] = event

	if _, err := manager.ApplyEvent(ctx, event.ID); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if repo.events[event.ID].Status != data.StatusSynced {
		t.Fatalf("event status = %q", repo.events[event.ID].Status)
	}
	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "skill", ResourceID: "s1", Relation: "editor", SubjectType: "user", SubjectID: "u1"})
	if err != nil || len(rels) != 1 {
		t.Fatalf("relationship not written, rels=%d err=%v", len(rels), err)
	}

	if _, err := manager.CompensateEvent(ctx, event.ID); err != nil {
		t.Fatalf("CompensateEvent returned error: %v", err)
	}
	rels, err = store.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "skill", ResourceID: "s1", Relation: "editor", SubjectType: "user", SubjectID: "u1"})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	if len(rels) != 0 {
		t.Fatalf("relationship still exists after compensate: %d", len(rels))
	}
}

func TestManagerApplyDelete(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	_, _ = store.WriteRelationships(ctx, authz.Relationship{
		Resource: authz.ObjectRef{Type: "git_repository", ID: "r1"},
		Relation: "writer",
		Subject:  authz.SubjectRef{Type: "user", ID: "u1"},
	})
	repo := newProjectionRepo()
	manager := NewManager(repo, store, nil)
	event, err := manager.NewDeleteEvent("grant", "grant-1", authz.RelationshipFilter{
		ResourceType: "git_repository",
		ResourceID:   "r1",
		Relation:     "writer",
		SubjectType:  "user",
		SubjectID:    "u1",
	}, authz.Relationship{
		Resource: authz.ObjectRef{Type: "git_repository", ID: "r1"},
		Relation: "writer",
		Subject:  authz.SubjectRef{Type: "user", ID: "u1"},
	})
	if err != nil {
		t.Fatalf("NewDeleteEvent returned error: %v", err)
	}
	repo.events[event.ID] = event

	if _, err := manager.ApplyEvent(ctx, event.ID); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "git_repository", ResourceID: "r1", Relation: "writer", SubjectType: "user", SubjectID: "u1"})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	if len(rels) != 0 {
		t.Fatalf("relationship still exists after delete projection: %d", len(rels))
	}
}

type projectionRepo struct {
	events map[string]*data.OutboxEventModel
}

func newProjectionRepo() *projectionRepo {
	return &projectionRepo{events: map[string]*data.OutboxEventModel{}}
}

func (r *projectionRepo) GetOutboxEvent(_ context.Context, id string) (*data.OutboxEventModel, error) {
	return r.events[id], nil
}

func (r *projectionRepo) UpdateOutboxEvent(_ context.Context, id string, columns map[string]any) error {
	event := r.events[id]
	if status, ok := columns["status"].(string); ok {
		event.Status = status
	}
	if lastError, ok := columns["last_error"].(string); ok {
		event.LastError = lastError
	}
	return nil
}
