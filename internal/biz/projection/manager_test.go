package projection

import (
	"context"
	"testing"
	"time"

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
		Resource: authz.ObjectRef{Type: "skill", ID: "s1"},
		Relation: "editor",
		Subject:  authz.SubjectRef{Type: "user", ID: "u1"},
	})
	repo := newProjectionRepo()
	manager := NewManager(repo, store, nil)
	event, err := manager.NewDeleteEvent("grant", "grant-1", authz.RelationshipFilter{
		ResourceType: "skill",
		ResourceID:   "s1",
		Relation:     "editor",
		SubjectType:  "user",
		SubjectID:    "u1",
	}, authz.Relationship{
		Resource: authz.ObjectRef{Type: "skill", ID: "s1"},
		Relation: "editor",
		Subject:  authz.SubjectRef{Type: "user", ID: "u1"},
	})
	if err != nil {
		t.Fatalf("NewDeleteEvent returned error: %v", err)
	}
	repo.events[event.ID] = event

	if _, err := manager.ApplyEvent(ctx, event.ID); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "skill", ResourceID: "s1", Relation: "editor", SubjectType: "user", SubjectID: "u1"})
	if err != nil {
		t.Fatalf("ReadRelationships returned error: %v", err)
	}
	if len(rels) != 0 {
		t.Fatalf("relationship still exists after delete projection: %d", len(rels))
	}
}

func TestManagerApplyBatchDelete(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	rels := []authz.Relationship{
		{Resource: authz.ObjectRef{Type: "role_binding", ID: "g1"}, Relation: "role", Subject: authz.SubjectRef{Type: "custom_role", ID: "skill:reviewer"}},
		{Resource: authz.ObjectRef{Type: "role_binding", ID: "g1"}, Relation: "grantee", Subject: authz.SubjectRef{Type: "user", ID: "u1"}},
		{Resource: authz.ObjectRef{Type: "skill", ID: "s1"}, Relation: "custom_binding", Subject: authz.SubjectRef{Type: "role_binding", ID: "g1"}},
	}
	if _, err := store.WriteRelationships(ctx, rels...); err != nil {
		t.Fatal(err)
	}
	repo := newProjectionRepo()
	manager := NewManager(repo, store, nil)
	filters := make([]authz.RelationshipFilter, 0, len(rels))
	for _, rel := range rels {
		filters = append(filters, exactFilter(rel))
	}
	event, err := manager.NewBatchDeleteEvent("grant", "g1", filters, rels...)
	if err != nil {
		t.Fatal(err)
	}
	repo.events[event.ID] = event
	if _, err := manager.ApplyEvent(ctx, event.ID); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil || len(got) != 0 {
		t.Fatalf("relationships = %#v, err = %v", got, err)
	}
}

func TestManagerReplaceAndCompensateRoleCapabilities(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	oldRels := []authz.Relationship{
		{Resource: authz.ObjectRef{Type: "custom_role", ID: "skill:reviewer"}, Relation: "view", Subject: authz.SubjectRef{Type: "user", ID: "*"}},
		{Resource: authz.ObjectRef{Type: "custom_role", ID: "skill:reviewer"}, Relation: "review", Subject: authz.SubjectRef{Type: "user", ID: "*"}},
	}
	newRels := []authz.Relationship{
		oldRels[0],
		{Resource: authz.ObjectRef{Type: "custom_role", ID: "skill:reviewer"}, Relation: "edit", Subject: authz.SubjectRef{Type: "user", ID: "*"}},
	}
	if _, err := store.WriteRelationships(ctx, oldRels...); err != nil {
		t.Fatal(err)
	}
	repo := newProjectionRepo()
	manager := NewManager(repo, store, nil)
	event, err := manager.NewReplaceEvent("role_template", "reviewer", oldRels, newRels)
	if err != nil {
		t.Fatal(err)
	}
	repo.events[event.ID] = event
	if _, err := manager.ApplyEvent(ctx, event.ID); err != nil {
		t.Fatal(err)
	}
	assertRelationshipCount(t, ctx, store, "review", 0)
	assertRelationshipCount(t, ctx, store, "edit", 1)
	if _, err := manager.CompensateEvent(ctx, event.ID); err != nil {
		t.Fatal(err)
	}
	assertRelationshipCount(t, ctx, store, "review", 1)
	assertRelationshipCount(t, ctx, store, "edit", 0)
}

func exactFilter(rel authz.Relationship) authz.RelationshipFilter {
	return authz.RelationshipFilter{ResourceType: rel.Resource.Type, ResourceID: rel.Resource.ID, Relation: rel.Relation, SubjectType: rel.Subject.Type, SubjectID: rel.Subject.ID, SubjectRel: rel.Subject.Relation}
}

func assertRelationshipCount(t *testing.T, ctx context.Context, store *authz.MemoryRelationshipStore, relation string, want int) {
	t.Helper()
	rels, err := store.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: "custom_role", ResourceID: "skill:reviewer", Relation: relation})
	if err != nil || len(rels) != want {
		t.Fatalf("relation %s count = %d, want %d, err = %v", relation, len(rels), want, err)
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
	if event == nil {
		return nil
	}
	if status, ok := columns["status"].(string); ok {
		event.Status = status
	}
	if lastError, ok := columns["last_error"].(string); ok {
		event.LastError = lastError
	}
	if rc, ok := columns["retry_count"].(int); ok {
		event.RetryCount = rc
	}
	if nra, ok := columns["next_run_at"]; ok {
		if t, ok2 := nra.(time.Time); ok2 && !t.IsZero() {
			event.NextRunAt = &t
		} else if nra == nil {
			event.NextRunAt = nil
		}
	}
	return nil
}

func (r *projectionRepo) ListOutboxEventsForRetry(_ context.Context, limit int) ([]data.OutboxEventModel, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now().UTC()
	out := make([]data.OutboxEventModel, 0, limit)
	for _, v := range r.events {
		if v == nil || v.RetryCount >= data.MaxOutboxRetries {
			continue
		}
		switch v.Status {
		case data.StatusPending, data.StatusSubmitted, data.StatusFailed:
		default:
			continue
		}
		if v.NextRunAt != nil && v.NextRunAt.After(now) {
			continue
		}
		out = append(out, *v)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *projectionRepo) IncrementOutboxRetry(_ context.Context, id string) (*data.OutboxEventModel, error) {
	event := r.events[id]
	if event == nil {
		return nil, nil
	}
	event.RetryCount++
	return event, nil
}

func TestManagerMarkFailedIncrementsRetryCountAndDeadLetters(t *testing.T) {
	store := authz.NewMemoryRelationshipStore()
	repo := newProjectionRepo()
	manager := NewManager(repo, store, nil)
	event, err := manager.NewWriteEvent("grant", "grant-1", authz.Relationship{
		Resource: authz.ObjectRef{Type: "skill", ID: "s1"}, Relation: "editor",
		Subject: authz.SubjectRef{Type: "user", ID: "u1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.events[event.ID] = event

	// Failing once should bump retry_count to 1 and keep status failed.
	if err := manager.markFailed(context.Background(), event.ID, errFake{}); err != nil {
		t.Fatalf("markFailed returned error: %v", err)
	}
	if got := repo.events[event.ID]; got == nil || got.RetryCount != 1 || got.Status != StatusFailed {
		t.Fatalf("after first failure: status=%q retry=%d", repo.events[event.ID].Status, repo.events[event.ID].RetryCount)
	}

	// Drive retries up to the dead-letter budget; the final failure should
	// dead-letter the event rather than keep retrying.
	for i := 1; i < data.MaxOutboxRetries; i++ {
		if err := manager.markFailed(context.Background(), event.ID, errFake{}); err != nil {
			t.Fatalf("markFailed #%d returned error: %v", i, err)
		}
	}
	if repo.events[event.ID].Status != data.StatusDead {
		t.Fatalf("event should be dead-lettered after %d retries, status=%q", data.MaxOutboxRetries, repo.events[event.ID].Status)
	}
	// A dead-lettered event must not be returned by ListOutboxEventsForRetry.
	due, err := repo.ListOutboxEventsForRetry(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range due {
		if e.ID == event.ID {
			t.Fatalf("dead-lettered event %s was returned for retry", event.ID)
		}
	}
}

type errFake struct{}

func (errFake) Error() string { return "synthetic failure" }
