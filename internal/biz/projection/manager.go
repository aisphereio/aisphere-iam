// Package projection coordinates IAM DB outbox events with SpiceDB projection.
package projection

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/dtmx"

	"github.com/aisphereio/aisphere-iam/internal/biz/idgen"
	"github.com/aisphereio/aisphere-iam/internal/data"
)

const (
	TopicAuthzProjection = "iam.authz.projection"

	OperationWrite   = "write"
	OperationDelete  = "delete"
	OperationReplace = "replace"

	StatusProjecting = "projecting"
	StatusFailed     = "failed"
)

type outboxRepo interface {
	GetOutboxEvent(ctx context.Context, id string) (*data.OutboxEventModel, error)
	UpdateOutboxEvent(ctx context.Context, id string, columns map[string]any) error
	ListOutboxEventsForRetry(ctx context.Context, limit int) ([]data.OutboxEventModel, error)
	IncrementOutboxRetry(ctx context.Context, id string) (*data.OutboxEventModel, error)
}

type Payload struct {
	Operation             string                     `json:"operation"`
	Relationships         []authz.Relationship       `json:"relationships,omitempty"`
	PreviousRelationships []authz.Relationship       `json:"previous_relationships,omitempty"`
	Filters               []authz.RelationshipFilter `json:"filters,omitempty"`
	Filter                authz.RelationshipFilter   `json:"filter,omitempty"`
}

type BranchPayload struct {
	EventID string `json:"event_id"`
}

type Manager struct {
	repo   outboxRepo
	writer authz.RelationshipWriter
	dtm    dtmx.Manager
	now    func() time.Time
}

func NewManager(repo outboxRepo, writer authz.RelationshipWriter, dtm dtmx.Manager) *Manager {
	return &Manager{repo: repo, writer: writer, dtm: dtm, now: func() time.Time { return time.Now().UTC() }}
}

func (m *Manager) NewWriteEvent(aggregateType, aggregateID string, rels ...authz.Relationship) (*data.OutboxEventModel, error) {
	if len(rels) == 0 {
		return nil, nil
	}
	return m.newEvent(aggregateType, aggregateID, Payload{Operation: OperationWrite, Relationships: rels})
}

func (m *Manager) NewDeleteEvent(aggregateType, aggregateID string, filter authz.RelationshipFilter, rels ...authz.Relationship) (*data.OutboxEventModel, error) {
	return m.newEvent(aggregateType, aggregateID, Payload{Operation: OperationDelete, Filter: filter, Relationships: rels})
}

func (m *Manager) NewBatchDeleteEvent(aggregateType, aggregateID string, filters []authz.RelationshipFilter, rels ...authz.Relationship) (*data.OutboxEventModel, error) {
	return m.newEvent(aggregateType, aggregateID, Payload{Operation: OperationDelete, Filters: filters, Relationships: rels})
}

func (m *Manager) NewReplaceEvent(aggregateType, aggregateID string, previous, desired []authz.Relationship) (*data.OutboxEventModel, error) {
	return m.newEvent(aggregateType, aggregateID, Payload{Operation: OperationReplace, PreviousRelationships: previous, Relationships: desired})
}

func (m *Manager) Dispatch(ctx context.Context, event *data.OutboxEventModel) (authz.WriteResult, error) {
	if event == nil {
		return authz.WriteResult{}, nil
	}
	if m == nil || m.writer == nil {
		return authz.WriteResult{}, errors.New("projection manager is not configured")
	}
	if m.dtm != nil && m.dtm.Enabled() {
		gid, err := m.dtm.NewGID(ctx)
		if err != nil {
			_ = m.markFailed(ctx, event.ID, err)
			return authz.WriteResult{}, err
		}
		payload := BranchPayload{EventID: event.ID}
		saga := dtmx.NewSaga(gid, TopicAuthzProjection).
			AddHTTP("project-authz",
				m.dtm.BranchURL("iam/projection/apply"),
				m.dtm.BranchURL("iam/projection/compensate"),
				payload,
			)
		if _, err := m.dtm.SubmitSaga(ctx, saga); err != nil {
			_ = m.markFailed(ctx, event.ID, err)
			return authz.WriteResult{}, err
		}
		return authz.WriteResult{}, nil
	}
	return m.ApplyEvent(ctx, event.ID)
}

func (m *Manager) ApplyEvent(ctx context.Context, eventID string) (authz.WriteResult, error) {
	event, payload, err := m.load(ctx, eventID)
	if err != nil {
		return authz.WriteResult{}, err
	}
	if event.Status == data.StatusSynced {
		return authz.WriteResult{}, nil
	}
	if err := m.repo.UpdateOutboxEvent(ctx, event.ID, map[string]any{"status": StatusProjecting}); err != nil {
		return authz.WriteResult{}, err
	}
	var wr authz.WriteResult
	switch payload.Operation {
	case OperationWrite:
		wr, err = m.writer.WriteRelationships(ctx, payload.Relationships...)
	case OperationDelete:
		filters := payload.Filters
		if len(filters) == 0 {
			filters = []authz.RelationshipFilter{payload.Filter}
		}
		wr, err = deleteRelationships(ctx, m.writer, filters)
	case OperationReplace:
		wr, err = deleteRelationships(ctx, m.writer, relationshipFilters(payload.PreviousRelationships))
		if err == nil && len(payload.Relationships) > 0 {
			part, writeErr := m.writer.WriteRelationships(ctx, payload.Relationships...)
			wr.Written += part.Written
			if part.ConsistencyToken != "" {
				wr.ConsistencyToken = part.ConsistencyToken
			}
			err = writeErr
		}
	default:
		err = errors.New("unsupported projection operation: " + payload.Operation)
	}
	if err != nil {
		_ = m.markFailed(ctx, event.ID, err)
		return wr, err
	}
	return wr, m.repo.UpdateOutboxEvent(ctx, event.ID, map[string]any{
		"status":         data.StatusSynced,
		"last_error":     "",
		"retry_count":    event.RetryCount,
		"next_run_at":    nil,
		"payload_json":   event.PayloadJSON,
		"aggregate_type": event.AggregateType,
	})
}

func (m *Manager) CompensateEvent(ctx context.Context, eventID string) (authz.WriteResult, error) {
	_, payload, err := m.load(ctx, eventID)
	if err != nil {
		return authz.WriteResult{}, err
	}
	switch payload.Operation {
	case OperationWrite:
		var wr authz.WriteResult
		for _, rel := range payload.Relationships {
			part, err := m.writer.DeleteRelationships(ctx, authz.RelationshipFilter{
				ResourceType: rel.Resource.Type,
				ResourceID:   rel.Resource.ID,
				Relation:     rel.Relation,
				SubjectType:  rel.Subject.Type,
				SubjectID:    rel.Subject.ID,
				SubjectRel:   rel.Subject.Relation,
			})
			wr.Deleted += part.Deleted
			if part.ConsistencyToken != "" {
				wr.ConsistencyToken = part.ConsistencyToken
			}
			if err != nil {
				_ = m.markFailed(ctx, eventID, err)
				return wr, err
			}
		}
		_ = m.repo.UpdateOutboxEvent(ctx, eventID, map[string]any{"status": data.StatusArchived})
		return wr, nil
	case OperationDelete:
		wr, err := m.writer.WriteRelationships(ctx, payload.Relationships...)
		if err != nil {
			_ = m.markFailed(ctx, eventID, err)
		}
		return wr, err
	case OperationReplace:
		wr, err := deleteRelationships(ctx, m.writer, relationshipFilters(payload.Relationships))
		if err != nil {
			_ = m.markFailed(ctx, eventID, err)
			return wr, err
		}
		if len(payload.PreviousRelationships) > 0 {
			part, writeErr := m.writer.WriteRelationships(ctx, payload.PreviousRelationships...)
			wr.Written += part.Written
			if part.ConsistencyToken != "" {
				wr.ConsistencyToken = part.ConsistencyToken
			}
			if writeErr != nil {
				_ = m.markFailed(ctx, eventID, writeErr)
				return wr, writeErr
			}
		}
		_ = m.repo.UpdateOutboxEvent(ctx, eventID, map[string]any{"status": data.StatusArchived})
		return wr, nil
	default:
		return authz.WriteResult{}, errors.New("unsupported projection operation: " + payload.Operation)
	}
}

func relationshipFilters(rels []authz.Relationship) []authz.RelationshipFilter {
	filters := make([]authz.RelationshipFilter, 0, len(rels))
	for _, rel := range rels {
		filters = append(filters, authz.RelationshipFilter{
			ResourceType: rel.Resource.Type, ResourceID: rel.Resource.ID, Relation: rel.Relation,
			SubjectType: rel.Subject.Type, SubjectID: rel.Subject.ID, SubjectRel: rel.Subject.Relation,
		})
	}
	return filters
}

func deleteRelationships(ctx context.Context, writer authz.RelationshipWriter, filters []authz.RelationshipFilter) (authz.WriteResult, error) {
	var out authz.WriteResult
	for _, filter := range filters {
		part, err := writer.DeleteRelationships(ctx, filter)
		out.Deleted += part.Deleted
		if part.ConsistencyToken != "" {
			out.ConsistencyToken = part.ConsistencyToken
		}
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func (m *Manager) newEvent(aggregateType, aggregateID string, payload Payload) (*data.OutboxEventModel, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	now := m.now()
	return &data.OutboxEventModel{
		ID:            idgen.New("outbox"),
		Topic:         TopicAuthzProjection,
		AggregateType: strings.TrimSpace(aggregateType),
		AggregateID:   strings.TrimSpace(aggregateID),
		PayloadJSON:   string(body),
		Status:        data.StatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (m *Manager) load(ctx context.Context, eventID string) (*data.OutboxEventModel, Payload, error) {
	if m == nil || m.repo == nil || m.writer == nil {
		return nil, Payload{}, errors.New("projection manager is not configured")
	}
	event, err := m.repo.GetOutboxEvent(ctx, strings.TrimSpace(eventID))
	if err != nil {
		return nil, Payload{}, err
	}
	var payload Payload
	if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
		return nil, Payload{}, err
	}
	return event, payload, nil
}

func (m *Manager) markFailed(ctx context.Context, eventID string, err error) error {
	if m == nil || m.repo == nil {
		return nil
	}
	// Atomically bump retry_count and read it back so we can dead-letter
	// events that have exhausted their retry budget instead of spinning
	// forever (the previous implementation hardcoded retry_count=1, which
	// both lost the true count and never dead-lettered).
	updated, incErr := m.repo.IncrementOutboxRetry(ctx, eventID)
	if incErr != nil {
		// Fall back to a plain status update if the increment helper is
		// unavailable so the failure is still recorded.
		return m.repo.UpdateOutboxEvent(ctx, eventID, map[string]any{
			"status":      StatusFailed,
			"last_error":  err.Error(),
			"next_run_at": m.now().Add(time.Minute),
		})
	}
	status := StatusFailed
	nextRun := m.now().Add(time.Minute)
	if updated != nil && updated.RetryCount >= data.MaxOutboxRetries {
		// Dead-letter: stop retrying automatically. next_run_at is cleared so
		// ListOutboxEventsForRetry (which also filters retry_count < budget)
		// never returns it again.
		status = data.StatusDead
		nextRun = time.Time{}
	}
	return m.repo.UpdateOutboxEvent(ctx, eventID, map[string]any{
		"status":      status,
		"last_error":  err.Error(),
		"next_run_at": nextRun,
	})
}

// RetryOnce re-dispatches up to limit due outbox events (pending, submitted,
// or failed whose next_run_at has elapsed and that have not exhausted their
// retry budget).  It mirrors IdentityProjectionDispatcher.RetryOnce so the
// iam_outbox_events projection layer gains the same automatic recovery as the
// directory projection layer.
func (m *Manager) RetryOnce(ctx context.Context, limit int) (int, error) {
	if m == nil || m.repo == nil {
		return 0, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	events, err := m.repo.ListOutboxEventsForRetry(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for i := range events {
		event := events[i]
		if _, err := m.Dispatch(ctx, &event); err != nil {
			_ = m.markFailed(ctx, event.ID, err)
			continue
		}
		processed++
	}
	return processed, nil
}

// StartRetryWorker polls the outbox at interval and re-dispatches due events
// until ctx is cancelled.  It is the iam_outbox_events counterpart to
// IdentityProjectionDispatcher.StartRetryWorker.
func (m *Manager) StartRetryWorker(ctx context.Context, interval time.Duration) {
	if m == nil || m.repo == nil {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.RetryOnce(ctx, 100)
		}
	}
}
