package data

import "context"

// UpsertProject keeps the in-memory repository behavior aligned with the
// PostgreSQL repository for business and service lifecycle tests.
func (r *MemoryControlPlaneRepository) UpsertProject(_ context.Context, project *ProjectModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	value := clone(project)
	if current := r.projects[value.ID]; current != nil && value.CreatedAt.IsZero() {
		value.CreatedAt = current.CreatedAt
	}
	value.CreatedAt = nowIfZero(value.CreatedAt)
	value.UpdatedAt = nowIfZero(value.UpdatedAt)
	r.projects[value.ID] = value
	return nil
}

func (r *MemoryControlPlaneRepository) CreateOutboxEvents(_ context.Context, events ...*OutboxEventModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, event := range events {
		r.saveEvent(event)
	}
	return nil
}

func (r *MemoryControlPlaneRepository) ListOutboxEvents(_ context.Context, opts ListOptions) ([]OutboxEventModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]OutboxEventModel, 0, len(r.events))
	for _, event := range r.events {
		if statusOK(event.Status, opts.Status) {
			out = append(out, *clone(event))
		}
	}
	return out, nil
}
