package service

import (
	"context"

	accessv1 "github.com/aisphereio/aisphere-iam/api/iam/access/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	accessbiz "github.com/aisphereio/aisphere-iam/internal/biz/accessquery"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AccessQueryService implements the proto AccessQueryServiceServer.
type AccessQueryService struct {
	accessv1.UnimplementedAccessQueryServiceServer
	biz  *accessbiz.Service
	repo data.ControlPlaneRepository
}

// NewAccessQueryService creates a new AccessQueryService.
func NewAccessQueryService(biz *accessbiz.Service, repo data.ControlPlaneRepository) *AccessQueryService {
	return &AccessQueryService{biz: biz, repo: repo}
}

// ListSubjectEntitlements returns all effective permissions for a subject.
func (s *AccessQueryService) ListSubjectEntitlements(ctx context.Context, req *accessv1.ListSubjectEntitlementsRequest) (*accessv1.ListSubjectEntitlementsReply, error) {
	orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}

	reply, err := s.biz.ListSubjectEntitlements(ctx, accessbiz.ListSubjectEntitlementsRequest{
		OrgID:        orgID,
		Subject:      accessbiz.SubjectRef{Type: req.GetSubject().GetType(), ID: req.GetSubject().GetId()},
		ResourceType: req.GetResourceType(),
		PageSize:     int(req.GetPageSize()),
		PageToken:    req.GetPageToken(),
	})
	if err != nil {
		return nil, err
	}

	out := make([]*accessv1.Entitlement, 0, len(reply.Entitlements))
	for _, e := range reply.Entitlements {
		out = append(out, entitlementToProto(e))
	}

	return &accessv1.ListSubjectEntitlementsReply{
		Entitlements:  out,
		NextPageToken: reply.NextPageToken,
		TotalSize:     reply.TotalSize,
	}, nil
}

// ListResourceAccess returns all subjects with effective access to a resource.
func (s *AccessQueryService) ListResourceAccess(ctx context.Context, req *accessv1.ListResourceAccessRequest) (*accessv1.ListResourceAccessReply, error) {
	orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}

	reply, err := s.biz.ListResourceAccess(ctx, accessbiz.ListResourceAccessRequest{
		OrgID:       orgID,
		Resource:    accessbiz.ResourceRef{Type: req.GetResource().GetType(), ID: req.GetResource().GetId()},
		SubjectType: req.GetSubjectType(),
		PageSize:    int(req.GetPageSize()),
		PageToken:   req.GetPageToken(),
	})
	if err != nil {
		return nil, err
	}

	out := make([]*accessv1.Entitlement, 0, len(reply.Entitlements))
	for _, e := range reply.Entitlements {
		out = append(out, entitlementToProto(e))
	}

	return &accessv1.ListResourceAccessReply{
		Entitlements:  out,
		NextPageToken: reply.NextPageToken,
		TotalSize:     reply.TotalSize,
	}, nil
}

// PreviewGrant previews what permissions a subject would receive.
func (s *AccessQueryService) PreviewGrant(ctx context.Context, req *accessv1.PreviewGrantRequest) (*accessv1.PreviewGrantReply, error) {
	orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
	if err != nil {
		return nil, err
	}

	reply, err := s.biz.PreviewGrant(ctx, accessbiz.PreviewGrantRequest{
		OrgID:    orgID,
		Resource: accessbiz.ResourceRef{Type: req.GetResource().GetType(), ID: req.GetResource().GetId()},
		RoleKey:  req.GetRoleKey(),
		Subject:  accessbiz.SubjectRef{Type: req.GetSubject().GetType(), ID: req.GetSubject().GetId()},
	})
	if err != nil {
		return nil, err
	}

	return &accessv1.PreviewGrantReply{
		Permissions:      reply.Permissions,
		AlreadyGranted:   reply.AlreadyGranted,
		ExistingGrantId:  reply.ExistingGrantID,
		ConsistencyToken: reply.ConsistencyToken,
	}, nil
}

// entitlementToProto converts a biz Entitlement to a proto Entitlement.
func entitlementToProto(e accessbiz.Entitlement) *accessv1.Entitlement {
	sourceType := accessv1.EntitlementSourceType_ENTITLEMENT_SOURCE_TYPE_UNSPECIFIED
	switch e.SourceType {
	case accessbiz.SourceDirectGrant:
		sourceType = accessv1.EntitlementSourceType_DIRECT_GRANT
	case accessbiz.SourceGroupGrant:
		sourceType = accessv1.EntitlementSourceType_GROUP_GRANT
	case accessbiz.SourceParentInheritance:
		sourceType = accessv1.EntitlementSourceType_PARENT_INHERITANCE
	case accessbiz.SourceOrgInheritance:
		sourceType = accessv1.EntitlementSourceType_ORG_INHERITANCE
	case accessbiz.SourcePlatformInheritance:
		sourceType = accessv1.EntitlementSourceType_PLATFORM_INHERITANCE
	}

	var ts *timestamppb.Timestamp
	if e.ExpiresAt != nil {
		ts = timestamppb.New(*e.ExpiresAt)
	}

	return &accessv1.Entitlement{
		Id:               e.ID,
		Subject:          &resourcev1.SubjectRef{Type: e.Subject.Type, Id: e.Subject.ID, Relation: e.Subject.Relation},
		Resource:         &resourcev1.ResourceRef{Type: e.Resource.Type, Id: e.Resource.ID},
		RoleKey:          e.RoleKey,
		Permissions:      e.Permissions,
		SourceType:       sourceType,
		SourceSubject:    &resourcev1.SubjectRef{Type: e.SourceSubject.Type, Id: e.SourceSubject.ID},
		SourceResource:   &resourcev1.ResourceRef{Type: e.SourceResource.Type, Id: e.SourceResource.ID},
		GrantId:          e.GrantID,
		RevocableHere:    e.RevocableHere,
		ExpiresAt:        ts,
		ConsistencyToken: e.ConsistencyToken,
	}
}