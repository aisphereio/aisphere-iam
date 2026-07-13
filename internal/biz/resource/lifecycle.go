package resource

import (
	"context"
	"errors"
	"strings"

	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/kernel/authz"
)

type UnbindResourceRequest struct {
	ID string
}

func (s *Service) UnbindResource(ctx context.Context, req UnbindResourceRequest) (*data.ResourceBindingModel, authz.WriteResult, error) {
	if s.repo == nil {
		return nil, authz.WriteResult{}, errors.New("resource service repository is nil")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return nil, authz.WriteResult{}, errors.New("binding_id is required")
	}
	binding, err := s.repo.GetResourceBinding(ctx, id)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if binding.Status == data.StatusArchived {
		return binding, authz.WriteResult{}, nil
	}
	rel, err := s.bindingRelationship(ctx, BindResourceRequest{
		Source:   ResourceRef{Type: binding.SourceType, ID: binding.SourceID},
		Relation: binding.Relation,
		Target:   ResourceRef{Type: binding.TargetType, ID: binding.TargetID},
	})
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	filter := authz.RelationshipFilter{
		ResourceType: rel.Resource.Type,
		ResourceID:   rel.Resource.ID,
		Relation:     rel.Relation,
		SubjectType:  rel.Subject.Type,
		SubjectID:    rel.Subject.ID,
		SubjectRel:   rel.Subject.Relation,
	}
	event, err := s.projection.NewDeleteEvent("resource_binding", binding.ID, filter, rel)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if err := s.repo.UnbindResource(ctx, binding.ID, event); err != nil {
		return nil, authz.WriteResult{}, err
	}
	binding.Status = data.StatusArchived
	wr, err := s.projection.Dispatch(ctx, event)
	return binding, wr, err
}
