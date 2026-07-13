package service

import (
	"context"

	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	resourcebiz "github.com/aisphereio/aisphere-iam/internal/biz/resource"
	"github.com/aisphereio/aisphere-iam/internal/data"
)

func (s *ResourceService) UnbindResource(ctx context.Context, req *resourcev1.UnbindResourceRequest) (*resourcev1.UnbindResourceReply, error) {
	binding, _, err := s.biz.UnbindResource(ctx, resourcebiz.UnbindResourceRequest{ID: req.GetBindingId()})
	if err != nil {
		return nil, err
	}
	return &resourcev1.UnbindResourceReply{BindingId: binding.ID, Unbound: binding.Status == data.StatusArchived}, nil
}

func (s *ResourceService) ListResourceBindings(ctx context.Context, req *resourcev1.ListResourceBindingsRequest) (*resourcev1.ListResourceBindingsReply, error) {
	source, target := req.GetSource(), req.GetTarget()
	page, err := s.repo.ListResourceBindings(ctx, data.ListOptions{
		ResourceType: source.GetType(), ResourceID: source.GetId(),
		TargetType: target.GetType(), TargetID: target.GetId(), Relation: req.GetRelation(), Status: req.GetStatus(),
		Page: pageFromToken(req.GetPageToken()), Size: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*resourcev1.ResourceBinding, 0, len(page.Items))
	for i := range page.Items {
		out = append(out, resourceBindingModelToProto(&page.Items[i]))
	}
	return &resourcev1.ListResourceBindingsReply{Bindings: out, TotalSize: page.Total, NextPageToken: nextPage(page)}, nil
}

func (s *ResourceService) ListExternalResourceBindings(ctx context.Context, req *resourcev1.ListExternalResourceBindingsRequest) (*resourcev1.ListExternalResourceBindingsReply, error) {
	resource := req.GetResource()
	page, err := s.repo.ListExternalResourceBindings(ctx, data.ListOptions{
		ResourceType: resource.GetType(), ResourceID: resource.GetId(), Provider: req.GetProvider(),
		ExternalType: req.GetExternalType(), ExternalID: req.GetExternalId(), Status: req.GetSyncStatus(),
		Page: pageFromToken(req.GetPageToken()), Size: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*resourcev1.ExternalResourceBinding, 0, len(page.Items))
	for i := range page.Items {
		out = append(out, externalBindingModelToProto(&page.Items[i]))
	}
	return &resourcev1.ListExternalResourceBindingsReply{Bindings: out, TotalSize: page.Total, NextPageToken: nextPage(page)}, nil
}
