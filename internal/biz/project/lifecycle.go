package project

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/aisphereio/kernel/authz"

	"github.com/aisphereio/aisphere-iam/internal/data"
)

type UpdateProjectRequest struct {
	ID              string
	ZoneID          string
	DisplayName     *string
	Description     *string
	Visibility      *string
	LabelsJSON      *string
	AnnotationsJSON *string
	MetadataJSON    *string
}

type ArchiveProjectRequest struct {
	ID     string
	ZoneID string
	Reason string
}

func (s *Service) GetProject(ctx context.Context, id, zoneID string) (*data.ProjectModel, error) {
	return s.loadProjectInZone(ctx, id, zoneID)
}

func (s *Service) UpdateProject(ctx context.Context, req UpdateProjectRequest) (*data.ProjectModel, error) {
	if s.repo == nil {
		return nil, errors.New("project service repository is nil")
	}
	project, err := s.loadProjectInZone(ctx, req.ID, req.ZoneID)
	if err != nil {
		return nil, err
	}
	if project.Status != data.StatusActive {
		return nil, errors.New("project is not active")
	}

	if req.DisplayName != nil {
		value := strings.TrimSpace(*req.DisplayName)
		if value == "" {
			return nil, errors.New("project display_name is required")
		}
		project.DisplayName = value
	}
	if req.Description != nil {
		project.Description = strings.TrimSpace(*req.Description)
	}
	if req.Visibility != nil {
		value := strings.TrimSpace(*req.Visibility)
		switch value {
		case "private", "org", "public":
			project.Visibility = value
		default:
			return nil, errors.New("project visibility must be private, org or public")
		}
	}
	if req.LabelsJSON != nil {
		value, err := normalizedJSONObject(*req.LabelsJSON)
		if err != nil {
			return nil, errors.New("project labels must be a JSON object")
		}
		project.LabelsJSON = value
	}
	if req.AnnotationsJSON != nil {
		value, err := normalizedJSONObject(*req.AnnotationsJSON)
		if err != nil {
			return nil, errors.New("project annotations must be a JSON object")
		}
		project.AnnotationsJSON = value
	}
	if req.MetadataJSON != nil {
		value, err := normalizedJSONObject(*req.MetadataJSON)
		if err != nil {
			return nil, errors.New("project metadata must be a JSON object")
		}
		project.MetadataJSON = value
	}

	project.UpdatedAt = s.now()
	if err := s.repo.UpsertProject(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}

func (s *Service) ArchiveProject(ctx context.Context, req ArchiveProjectRequest) (*data.ProjectModel, error) {
	if s.repo == nil {
		return nil, errors.New("project service repository is nil")
	}
	project, err := s.loadProjectInZone(ctx, req.ID, req.ZoneID)
	if err != nil {
		return nil, err
	}
	if project.Status == data.StatusArchived {
		return project, nil
	}
	if project.Status == data.StatusDeleted {
		return nil, errors.New("deleted project cannot be archived")
	}
	project.Status = data.StatusArchived
	project.UpdatedAt = s.now()
	if err := s.repo.UpsertProject(ctx, project); err != nil {
		return nil, err
	}
	// Purge the project's SpiceDB relationships so an archived project
	// immediately loses all grants (project#owner/developer/etc. and any
	// edges where the project is a subject).  Compensation data is captured
	// so a future un-archive could restore authorization.
	filters := []authz.RelationshipFilter{
		{ResourceType: ResourceTypeProject, ResourceID: project.ID},
		{SubjectType: ResourceTypeProject, SubjectID: project.ID},
	}
	rels, _ := s.captureRelationships(ctx, filters)
	if event, err := s.projection.NewBatchDeleteEvent("project", project.ID, filters, rels...); err == nil && event != nil {
		if err := s.repo.CreateOutboxEvents(ctx, event); err == nil {
			_, _ = s.projection.Dispatch(ctx, event)
		}
	}
	return project, nil
}

// captureRelationships reads current SpiceDB relationships for the given
// filters so they can be attached as compensation data to delete events.
// Best-effort: a missing reader or read error yields nil compensation.
func (s *Service) captureRelationships(ctx context.Context, filters []authz.RelationshipFilter) ([]authz.Relationship, error) {
	if s.reader == nil {
		return nil, nil
	}
	var captured []authz.Relationship
	for _, filter := range filters {
		if strings.TrimSpace(filter.ResourceType) == "" {
			continue
		}
		rels, err := s.reader.ReadRelationships(ctx, filter)
		if err != nil {
			return captured, err
		}
		captured = append(captured, rels...)
	}
	return captured, nil
}

func (s *Service) loadProjectInZone(ctx context.Context, id, zoneID string) (*data.ProjectModel, error) {
	if s.repo == nil {
		return nil, errors.New("project service repository is nil")
	}
	id = strings.TrimSpace(id)
	zoneID = strings.TrimSpace(zoneID)
	if id == "" || zoneID == "" {
		return nil, errors.New("project_id and zone_id are required")
	}
	project, err := s.repo.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(project.OrgID) != zoneID {
		return nil, errors.New("project does not belong to the current zone")
	}
	return project, nil
}

func normalizedJSONObject(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}", nil
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil || value == nil {
		return "", errors.New("invalid JSON object")
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}
