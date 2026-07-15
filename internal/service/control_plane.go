package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	grantv1 "github.com/aisphereio/aisphere-iam/api/iam/grant/v1"
	projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	grantbiz "github.com/aisphereio/aisphere-iam/internal/biz/grant"
	projectbiz "github.com/aisphereio/aisphere-iam/internal/biz/project"
	resourcebiz "github.com/aisphereio/aisphere-iam/internal/biz/resource"
"github.com/aisphereio/aisphere-iam/internal/data"
		"github.com/aisphereio/kernel/authn"
		"github.com/aisphereio/kernel/authz"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProjectService struct {
	projectv1.UnimplementedProjectServiceServer
	biz  *projectbiz.Service
	repo data.ControlPlaneRepository
}

func NewProjectService(biz *projectbiz.Service, repo data.ControlPlaneRepository) *ProjectService {
	return &ProjectService{biz: biz, repo: repo}
}

func (s *ProjectService) CreateProject(ctx context.Context, req *projectv1.CreateProjectRequest) (*projectv1.Project, error) {
		orgID, actor, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		project, _, err := s.biz.CreateProject(ctx, projectbiz.CreateProjectRequest{
			ZoneID: orgID, Slug: req.GetSlug(), DisplayName: req.GetDisplayName(), Description: req.GetDescription(),
			Visibility: visibilityToStatus(req.GetVisibility()), LabelsJSON: mapStringToJSON(req.GetLabels()), AnnotationsJSON: mapStringToJSON(req.GetAnnotations()), MetadataJSON: structToJSON(req.GetMetadata(), "{}"),
			CreatedBy: actor, Owner: actor,
		})
		if err != nil {
			return nil, err
		}
		return projectModelToProto(project), nil
	}

	func (s *ProjectService) ListProjects(ctx context.Context, req *projectv1.ListProjectsRequest) (*projectv1.ListProjectsReply, error) {
		orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		page, err := s.repo.ListProjects(ctx, data.ListOptions{OrgID: orgID, Q: req.GetQuery(), Status: lifecycleToStatus(req.GetStatus()), Page: pageFromToken(req.GetPageToken()), Size: int(req.GetPageSize())})
		if err != nil {
			return nil, err
		}
		out := make([]*projectv1.Project, 0, len(page.Items))
		for i := range page.Items {
			out = append(out, projectModelToProto(&page.Items[i]))
		}
		return &projectv1.ListProjectsReply{Projects: out, TotalSize: page.Total, NextPageToken: nextPage(page)}, nil
	}

func (s *ProjectService) RegisterCapability(ctx context.Context, req *projectv1.RegisterCapabilityRequest) (*projectv1.Capability, error) {
	in := req.GetCapability()
	capability, err := s.biz.RegisterCapability(ctx, projectbiz.RegisterCapabilityRequest{ID: in.GetId(), Name: in.GetName(), DisplayName: in.GetDisplayName(), OwnerService: in.GetOwnerService(), ConfigSchema: structToJSON(in.GetConfigSchema(), "{}")})
	if err != nil {
		return nil, err
	}
	return capabilityModelToProto(capability), nil
}

func (s *ProjectService) ListCapabilities(ctx context.Context, req *projectv1.ListCapabilitiesRequest) (*projectv1.ListCapabilitiesReply, error) {
	items, err := s.repo.ListCapabilities(ctx, data.ListOptions{Status: lifecycleToStatus(req.GetStatus())})
	if err != nil {
		return nil, err
	}
	out := make([]*projectv1.Capability, 0, len(items))
	for i := range items {
		out = append(out, capabilityModelToProto(&items[i]))
	}
	return &projectv1.ListCapabilitiesReply{Capabilities: out}, nil
}

type ResourceService struct {
	resourcev1.UnimplementedResourceServiceServer
	biz  *resourcebiz.Service
	repo data.ControlPlaneRepository
}

func NewResourceService(biz *resourcebiz.Service, repo data.ControlPlaneRepository) *ResourceService {
	return &ResourceService{biz: biz, repo: repo}
}

func (s *ResourceService) RegisterResourceType(ctx context.Context, req *resourcev1.RegisterResourceTypeRequest) (*resourcev1.ResourceType, error) {
	in := req.GetResourceType()
	rt, err := s.biz.RegisterResourceType(ctx, resourcebiz.RegisterResourceTypeRequest{
		Type: in.GetType(), CapabilityID: in.GetCapabilityId(), OwnerService: in.GetOwnerService(), ParentTypesJSON: stringSliceToJSON(in.GetParentTypes()),
		Grantable: in.GetGrantable(), Auditable: in.GetAuditable(), SpiceDBType: in.GetSpicedbType(), RelationsJSON: stringSliceToJSON(in.GetRelations()),
		PermissionsJSON: stringSliceToJSON(in.GetPermissions()), MetadataSchema: structToJSON(in.GetMetadata(), "{}"),
	})
	if err != nil {
		return nil, err
	}
	return resourceTypeModelToProto(rt), nil
}

func (s *ResourceService) GetResourceType(ctx context.Context, req *resourcev1.GetResourceTypeRequest) (*resourcev1.ResourceType, error) {
	rt, err := s.repo.GetResourceType(ctx, req.GetType())
	if err != nil {
		return nil, err
	}
	return resourceTypeModelToProto(rt), nil
}

func (s *ResourceService) ListResourceTypes(ctx context.Context, req *resourcev1.ListResourceTypesRequest) (*resourcev1.ListResourceTypesReply, error) {
	items, err := s.repo.ListResourceTypes(ctx, data.ListOptions{CapabilityID: req.GetCapabilityId(), Status: req.GetStatus()})
	if err != nil {
		return nil, err
	}
	out := make([]*resourcev1.ResourceType, 0, len(items))
	for i := range items {
		out = append(out, resourceTypeModelToProto(&items[i]))
	}
	return &resourcev1.ListResourceTypesReply{ResourceTypes: out}, nil
}

func (s *ResourceService) UpsertResource(ctx context.Context, req *resourcev1.UpsertResourceRequest) (*resourcev1.Resource, error) {
		actor, err := currentResourceSubject(ctx)
		if err != nil {
			return nil, err
		}
		orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		in := req.GetResource()
		model, _, err := s.biz.UpsertResource(ctx, resourcebiz.UpsertResourceRequest{
			Ref: resourceRef(in.GetRef()), OrgID: orgID, ProjectID: in.GetProjectId(), Parent: resourceRef(in.GetParent()),
			OwnerService: in.GetOwnerService(), OwnerResourceID: in.GetOwnerResourceId(), Slug: in.GetSlug(), DisplayName: in.GetDisplayName(),
			Path: in.GetPath(), Status: in.GetStatus(), Visibility: in.GetVisibility(), LabelsJSON: mapStringToJSON(in.GetLabels()),
			AnnotationsJSON: mapStringToJSON(in.GetAnnotations()), MetadataJSON: structToJSON(in.GetMetadata(), "{}"),
			CreatedBy: actor, Owner: resourceSubjectOr(req.GetOwner(), actor),
		})
		if err != nil {
			return nil, err
		}
		return resourceModelToProto(model), nil
	}

func (s *ResourceService) GetResource(ctx context.Context, req *resourcev1.GetResourceRequest) (*resourcev1.Resource, error) {
			zoneID, _, err := currentProjectContext(ctx, req.GetOrgId())
			if err != nil {
				return nil, err
			}
			model, err := s.repo.GetResource(ctx, req.GetResourceType(), req.GetResourceId(), zoneID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(model.OrgID) != zoneID {
			return nil, errors.New("resource does not belong to the current zone")
		}
		return resourceModelToProto(model), nil
	}

func (s *ResourceService) ListResources(ctx context.Context, req *resourcev1.ListResourcesRequest) (*resourcev1.ListResourcesReply, error) {
		orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		page, err := s.repo.ListResources(ctx, data.ListOptions{Type: req.GetType(), OrgID: orgID, ProjectID: req.GetProjectId(), Status: req.GetStatus(), Page: pageFromToken(req.GetPageToken()), Size: int(req.GetPageSize())})
	if err != nil {
		return nil, err
	}
	out := make([]*resourcev1.Resource, 0, len(page.Items))
	for i := range page.Items {
		out = append(out, resourceModelToProto(&page.Items[i]))
	}
	return &resourcev1.ListResourcesReply{Resources: out, TotalSize: page.Total, NextPageToken: nextPage(page)}, nil
}

func (s *ResourceService) MoveResource(ctx context.Context, req *resourcev1.MoveResourceRequest) (*resourcev1.Resource, error) {
			orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
			if err != nil {
				return nil, err
			}
			ref := resourceRef(&resourcev1.ResourceRef{Type: req.GetResourceType(), Id: req.GetResourceId()})
			var newParent resourcebiz.ResourceRef
			if p := req.GetNewParent(); p != nil {
				newParent = resourceRef(p)
			}
			if _, err := s.biz.MoveResource(ctx, ref, newParent, orgID); err != nil {
				return nil, err
			}
			return s.GetResource(ctx, &resourcev1.GetResourceRequest{OrgId: req.GetOrgId(), ResourceType: req.GetResourceType(), ResourceId: req.GetResourceId()})
		}

func (s *ResourceService) ArchiveResource(ctx context.Context, req *resourcev1.ArchiveResourceRequest) (*resourcev1.Resource, error) {
			orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
			if err != nil {
				return nil, err
			}
			if err := s.biz.ArchiveResource(ctx, resourceRef(&resourcev1.ResourceRef{Type: req.GetResourceType(), Id: req.GetResourceId()}), orgID); err != nil {
				return nil, err
			}
			return s.GetResource(ctx, &resourcev1.GetResourceRequest{OrgId: req.GetOrgId(), ResourceType: req.GetResourceType(), ResourceId: req.GetResourceId()})
		}

func (s *ResourceService) DeleteResource(ctx context.Context, req *resourcev1.DeleteResourceRequest) (*resourcev1.DeleteResourceReply, error) {
			orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
			if err != nil {
				return nil, err
			}
			if err := s.biz.DeleteResource(ctx, resourceRef(&resourcev1.ResourceRef{Type: req.GetResourceType(), Id: req.GetResourceId()}), orgID); err != nil {
				return nil, err
			}
			return &resourcev1.DeleteResourceReply{Ref: &resourcev1.ResourceRef{Type: req.GetResourceType(), Id: req.GetResourceId()}, Deleted: true}, nil
		}

func (s *ResourceService) BindResource(ctx context.Context, req *resourcev1.BindResourceRequest) (*resourcev1.ResourceBinding, error) {
		actor, err := currentResourceSubject(ctx)
		if err != nil {
			return nil, err
		}
		orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		in := req.GetBinding()
		model, _, err := s.biz.BindResource(ctx, resourcebiz.BindResourceRequest{ID: in.GetId(), OrgID: orgID, Source: resourceRef(in.GetSource()), Relation: in.GetRelation(), Target: resourceRef(in.GetTarget()), CreatedBy: actor})
		if err != nil {
			return nil, err
		}
		return resourceBindingModelToProto(model), nil
	}

func (s *ResourceService) UnbindResource(ctx context.Context, req *resourcev1.UnbindResourceRequest) (*resourcev1.UnbindResourceReply, error) {
			orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
			if err != nil {
				return nil, err
			}
			if err := s.biz.UnbindResource(ctx, req.GetBindingId(), orgID); err != nil {
			return nil, err
		}
		return &resourcev1.UnbindResourceReply{BindingId: req.GetBindingId(), Unbound: true}, nil
	}

func (s *ResourceService) ListResourceBindings(ctx context.Context, req *resourcev1.ListResourceBindingsRequest) (*resourcev1.ListResourceBindingsReply, error) {
		source := req.GetSource()
		orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		items, err := s.repo.ListResourceBindings(ctx, data.ListOptions{OrgID: orgID, ResourceType: source.GetType(), ResourceID: source.GetId(), Status: req.GetStatus()})
		if err != nil {
			return nil, err
		}
		out := make([]*resourcev1.ResourceBinding, 0, len(items))
		for i := range items {
			out = append(out, resourceBindingModelToProto(&items[i]))
		}
		return &resourcev1.ListResourceBindingsReply{Bindings: out, TotalSize: int64(len(out))}, nil
	}

func (s *ResourceService) BindExternalResource(ctx context.Context, req *resourcev1.BindExternalResourceRequest) (*resourcev1.ExternalResourceBinding, error) {
		in := req.GetBinding()
		orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		model, err := s.biz.BindExternalResource(ctx, resourcebiz.BindExternalResourceRequest{ID: in.GetId(), OrgID: orgID, Resource: resourceRef(in.GetResource()), Provider: in.GetProvider(), ExternalType: in.GetExternalType(), ExternalID: in.GetExternalId(), ExternalPath: in.GetExternalPath(), ExternalURL: in.GetExternalUrl(), SyncMode: in.GetSyncMode(), MetadataJSON: structToJSON(in.GetMetadata(), "{}")})
		if err != nil {
			return nil, err
		}
		return externalBindingModelToProto(model), nil
	}

func (s *ResourceService) ListExternalResourceBindings(ctx context.Context, req *resourcev1.ListExternalResourceBindingsRequest) (*resourcev1.ListExternalResourceBindingsReply, error) {
		source := req.GetResource()
		orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		items, err := s.repo.ListExternalResourceBindings(ctx, data.ListOptions{OrgID: orgID, ResourceType: source.GetType(), ResourceID: source.GetId(), Type: req.GetProvider(), Status: req.GetSyncStatus()})
		if err != nil {
			return nil, err
		}
		out := make([]*resourcev1.ExternalResourceBinding, 0, len(items))
		for i := range items {
			out = append(out, externalBindingModelToProto(&items[i]))
		}
		return &resourcev1.ListExternalResourceBindingsReply{Bindings: out, TotalSize: int64(len(out))}, nil
	}

type GrantService struct {
	grantv1.UnimplementedGrantServiceServer
	biz  *grantbiz.Service
	repo data.ControlPlaneRepository
}

func NewGrantService(biz *grantbiz.Service, repo data.ControlPlaneRepository) *GrantService {
	return &GrantService{biz: biz, repo: repo}
}

func (s *GrantService) RegisterRoleTemplate(ctx context.Context, req *grantv1.RegisterRoleTemplateRequest) (*grantv1.RoleTemplate, error) {
		in := req.GetRoleTemplate()
		actor, _ := currentGrantSubject(ctx)
		orgID, _ := currentOrgID(ctx)
		if in.GetBuiltIn() {
			orgID = ""
		}
		role, err := s.biz.RegisterRoleTemplate(ctx, grantbiz.RegisterRoleTemplateRequest{ID: in.GetId(), OrgID: orgID, ResourceType: in.GetResourceType(), RoleKey: in.GetRoleKey(), DisplayName: in.GetDisplayName(), Description: in.GetDescription(), Relation: in.GetRelation(), BuiltIn: in.GetBuiltIn(), Enabled: in.GetEnabled(), SortOrder: int(in.GetSortOrder()), MetadataJSON: structToJSON(in.GetMetadata(), "{}"), Permissions: in.GetPermissions(), Actor: actor})
		if err != nil {
			return nil, err
		}
		return roleTemplateModelToProto(role), nil
	}

func (s *GrantService) UpdateRoleTemplate(ctx context.Context, req *grantv1.UpdateRoleTemplateRequest) (*grantv1.RoleTemplate, error) {
	actor, _ := currentGrantSubject(ctx)
	role, err := s.biz.UpdateRoleTemplate(ctx, grantbiz.UpdateRoleTemplateRequest{
		ID: req.GetId(), DisplayName: req.GetDisplayName(), Description: req.GetDescription(),
		Permissions: req.GetPermissions(), ExpectedVersion: req.GetExpectedVersion(), Actor: actor,
	})
	if err != nil {
		return nil, err
	}
	return roleTemplateModelToProto(role), nil
}

func (s *GrantService) DisableRoleTemplate(ctx context.Context, req *grantv1.DisableRoleTemplateRequest) (*grantv1.RoleTemplate, error) {
	actor, _ := currentGrantSubject(ctx)
	role, err := s.biz.DisableRoleTemplate(ctx, grantbiz.DisableRoleTemplateRequest{
		ID: req.GetId(), ExpectedVersion: req.GetExpectedVersion(), ConfirmActiveGrants: req.GetConfirmActiveGrants(), Actor: actor,
	})
	if err != nil {
		return nil, err
	}
	return roleTemplateModelToProto(role), nil
}

func (s *GrantService) PreviewRoleTemplateImpact(ctx context.Context, req *grantv1.PreviewRoleTemplateImpactRequest) (*grantv1.PreviewRoleTemplateImpactReply, error) {
	impact, err := s.biz.PreviewRoleTemplateImpact(ctx, req.GetId(), req.GetPermissions())
	if err != nil {
		return nil, err
	}
	return &grantv1.PreviewRoleTemplateImpactReply{ActiveGrantCount: impact.ActiveGrantCount, AddedPermissions: impact.AddedPermissions, RemovedPermissions: impact.RemovedPermissions}, nil
}

func (s *GrantService) ListRoleTemplates(ctx context.Context, req *grantv1.ListRoleTemplatesRequest) (*grantv1.ListRoleTemplatesReply, error) {
	items, err := s.repo.ListRoleTemplates(ctx, req.GetResourceType())
	if err != nil {
		return nil, err
	}
	out := make([]*grantv1.RoleTemplate, 0, len(items))
	for i := range items {
		if (req.GetRoleKey() == "" || items[i].RoleKey == req.GetRoleKey()) && (req.Enabled == nil || items[i].Enabled == req.GetEnabled()) {
			out = append(out, roleTemplateModelToProto(&items[i]))
		}
	}
	return &grantv1.ListRoleTemplatesReply{RoleTemplates: out}, nil
}

func (s *GrantService) GrantAccess(ctx context.Context, req *grantv1.GrantAccessRequest) (*grantv1.Grant, error) {
		actor, err := currentGrantSubject(ctx)
		if err != nil {
			return nil, err
		}
		orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		grant, wr, err := s.biz.GrantAccess(ctx, grantbiz.GrantAccessRequest{OrgID: orgID, Resource: grantResource(req.GetResource()), RoleKey: req.GetRoleKey(), Subject: grantSubject(req.GetSubject()), Source: req.GetSource(), Reason: req.GetReason(), ExpiresAt: timestampPtr(req.GetExpiresAt()), CreatedBy: actor})
		if err != nil {
			return nil, err
		}
		out := grantModelToProto(grant)
		out.ConsistencyToken = wr.ConsistencyToken
		return out, nil
	}

func (s *GrantService) RevokeAccess(ctx context.Context, req *grantv1.RevokeAccessRequest) (*grantv1.RevokeAccessReply, error) {
			actor, err := currentGrantSubject(ctx)
			if err != nil {
				return nil, err
			}
			orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
			if err != nil {
				return nil, err
			}
			wr, err := s.biz.RevokeAccess(ctx, grantbiz.RevokeAccessRequest{GrantID: req.GetGrantId(), OrgID: orgID, Reason: req.GetReason(), DeleteGraphRelationship: true, Actor: actor})
		if err != nil {
			return nil, err
		}
		return &grantv1.RevokeAccessReply{GrantId: req.GetGrantId(), Revoked: true, ConsistencyToken: wr.ConsistencyToken}, nil
	}

func (s *GrantService) ListGrants(ctx context.Context, req *grantv1.ListGrantsRequest) (*grantv1.ListGrantsReply, error) {
		res := req.GetResource()
		sub := req.GetSubject()
		orgID, _, err := currentProjectContext(ctx, req.GetOrgId())
		if err != nil {
			return nil, err
		}
		page, err := s.repo.ListGrants(ctx, data.ListOptions{
			OrgID:        orgID,
			ResourceType: res.GetType(),
			ResourceID:   res.GetId(),
			SubjectType:  sub.GetType(),
			SubjectID:    sub.GetId(),
			Relation:     req.GetRelation(),
			RoleKey:      req.GetRoleKey(),
			Source:       req.GetSource(),
			Active:       req.Active,
			Page:         pageFromToken(req.GetPageToken()),
			Size:         int(req.GetPageSize()),
		})
		if err != nil {
			return nil, err
		}
		out := make([]*grantv1.Grant, 0, len(page.Items))
		for i := range page.Items {
			out = append(out, grantModelToProto(&page.Items[i]))
		}
		return &grantv1.ListGrantsReply{Grants: out, TotalSize: page.Total, NextPageToken: nextPage(page)}, nil
	}

func (s *GrantService) ExplainAccess(ctx context.Context, req *grantv1.ExplainAccessRequest) (*grantv1.ExplainAccessReply, error) {
		if _, _, err := currentProjectContext(ctx, req.GetOrgId()); err != nil {
			return nil, err
		}
		reply, err := s.biz.ExplainAccess(ctx, grantbiz.ExplainAccessRequest{Resource: grantResource(req.GetResource()), Permission: req.GetPermission(), Subject: grantSubject(req.GetSubject())})
		if err != nil {
			return nil, err
		}
		steps := make([]*grantv1.ExplainStep, 0, len(reply.Steps))
		for _, step := range reply.Steps {
			steps = append(steps, &grantv1.ExplainStep{
				Source:   step.Source,
				Relation: step.Relation,
				Resource: &resourcev1.ResourceRef{Type: step.Resource.Type, Id: step.Resource.ID},
				Subject:  &resourcev1.SubjectRef{Type: step.Subject.Type, Id: step.Subject.ID, Relation: step.Subject.Relation},
				Reason:   step.Reason,
			})
		}
		return &grantv1.ExplainAccessReply{Allowed: reply.Allowed, Effect: reply.Effect, Reason: reply.Reason, ConsistencyToken: reply.ConsistencyToken, Steps: steps}, nil
	}

func projectModelToProto(in *data.ProjectModel) *projectv1.Project {
	if in == nil {
		return nil
	}
	return &projectv1.Project{Id: in.ID, OrgId: in.OrgID, Slug: in.Slug, DisplayName: in.DisplayName, Description: in.Description, Status: statusToLifecycle(in.Status), Visibility: statusToVisibility(in.Visibility), Labels: jsonToStringMap(in.LabelsJSON), Annotations: jsonToStringMap(in.AnnotationsJSON), Metadata: jsonToStruct(in.MetadataJSON), CreatedBy: projectSubjectStringToProto(in.CreatedBy), CreatedAt: ts(in.CreatedAt), UpdatedAt: ts(in.UpdatedAt)}
}

func capabilityModelToProto(in *data.CapabilityModel) *projectv1.Capability {
	if in == nil {
		return nil
	}
	return &projectv1.Capability{Id: in.ID, Name: in.Name, DisplayName: in.DisplayName, OwnerService: in.OwnerService, Status: statusToLifecycle(in.Status), ConfigSchema: jsonToStruct(in.ConfigSchema), CreatedAt: ts(in.CreatedAt), UpdatedAt: ts(in.UpdatedAt)}
}

func projectCapabilityModelToProto(in *data.ProjectCapabilityModel) *projectv1.ProjectCapability {
	if in == nil {
		return nil
	}
	return &projectv1.ProjectCapability{ProjectId: in.ProjectID, CapabilityId: in.CapabilityID, Enabled: in.Enabled, Config: jsonToStruct(in.ConfigJSON), Quota: jsonToStruct(in.QuotaJSON), CreatedAt: ts(in.CreatedAt), UpdatedAt: ts(in.UpdatedAt)}
}

func resourceTypeModelToProto(in *data.ResourceTypeModel) *resourcev1.ResourceType {
	if in == nil {
		return nil
	}
	return &resourcev1.ResourceType{Type: in.Type, CapabilityId: in.CapabilityID, OwnerService: in.OwnerService, ParentTypes: jsonToStringSlice(in.ParentTypesJSON), Grantable: in.Grantable, Auditable: in.Auditable, SpicedbType: in.SpiceDBType, Relations: jsonToStringSlice(in.RelationsJSON), Permissions: jsonToStringSlice(in.PermissionsJSON), Metadata: jsonToStruct(in.MetadataSchema), Status: in.Status, CreatedAt: ts(in.CreatedAt), UpdatedAt: ts(in.UpdatedAt)}
}

func resourceModelToProto(in *data.ResourceModel) *resourcev1.Resource {
	if in == nil {
		return nil
	}
	return &resourcev1.Resource{Ref: &resourcev1.ResourceRef{Type: in.Type, Id: in.ID}, OrgId: in.OrgID, ProjectId: in.ProjectID, Parent: &resourcev1.ResourceRef{Type: in.ParentType, Id: in.ParentID}, OwnerService: in.OwnerService, OwnerResourceId: in.OwnerResourceID, Slug: in.Slug, DisplayName: in.DisplayName, Path: in.Path, Status: in.Status, Visibility: in.Visibility, Labels: jsonToStringMap(in.LabelsJSON), Annotations: jsonToStringMap(in.AnnotationsJSON), Metadata: jsonToStruct(in.MetadataJSON), CreatedAt: ts(in.CreatedAt), UpdatedAt: ts(in.UpdatedAt)}
}

func resourceBindingModelToProto(in *data.ResourceBindingModel) *resourcev1.ResourceBinding {
	if in == nil {
		return nil
	}
	return &resourcev1.ResourceBinding{Id: in.ID, Source: &resourcev1.ResourceRef{Type: in.SourceType, Id: in.SourceID}, Relation: in.Relation, Target: &resourcev1.ResourceRef{Type: in.TargetType, Id: in.TargetID}, Status: in.Status, CreatedAt: ts(in.CreatedAt), UpdatedAt: ts(in.UpdatedAt)}
}

func externalBindingModelToProto(in *data.ExternalResourceBindingModel) *resourcev1.ExternalResourceBinding {
	if in == nil {
		return nil
	}
	return &resourcev1.ExternalResourceBinding{Id: in.ID, Resource: &resourcev1.ResourceRef{Type: in.ResourceType, Id: in.ResourceID}, Provider: in.Provider, ExternalType: in.ExternalType, ExternalId: in.ExternalID, ExternalPath: in.ExternalPath, ExternalUrl: in.ExternalURL, SyncMode: in.SyncMode, SyncStatus: in.SyncStatus, Metadata: jsonToStruct(in.MetadataJSON)}
}

func roleTemplateModelToProto(in *data.RoleTemplateModel) *grantv1.RoleTemplate {
	if in == nil {
		return nil
	}
	return &grantv1.RoleTemplate{Id: in.ID, ResourceType: in.ResourceType, RoleKey: in.RoleKey, DisplayName: in.DisplayName, Description: in.Description, Relation: in.Relation, BuiltIn: in.BuiltIn, Enabled: in.Enabled, SortOrder: int32(in.SortOrder), Metadata: jsonToStruct(in.MetadataJSON), CreatedAt: ts(in.CreatedAt), UpdatedAt: ts(in.UpdatedAt), Permissions: append([]string(nil), in.Permissions...), ActiveGrantCount: in.ActiveGrantCount, Version: in.Version}
}

func grantModelToProto(in *data.GrantModel) *grantv1.Grant {
	if in == nil {
		return nil
	}
	return &grantv1.Grant{Id: in.ID, Resource: &resourcev1.ResourceRef{Type: in.ResourceType, Id: in.ResourceID}, RoleKey: in.RoleKey, Relation: in.Relation, Subject: &resourcev1.SubjectRef{Type: in.SubjectType, Id: in.SubjectID, Relation: in.SubjectRelation}, Source: in.Source, Reason: in.Reason, ExpiresAt: tsPtr(in.ExpiresAt), CreatedBy: &resourcev1.SubjectRef{Type: in.CreatedByType, Id: in.CreatedByID}, CreatedAt: ts(in.CreatedAt), RevokedAt: tsPtr(in.RevokedAt)}
}

func resourceSubject(in *resourcev1.SubjectRef) resourcebiz.SubjectRef {
	if in == nil {
		return resourcebiz.SubjectRef{}
	}
	return resourcebiz.SubjectRef{Type: in.GetType(), ID: in.GetId(), Relation: in.GetRelation()}
}
func grantSubject(in *resourcev1.SubjectRef) grantbiz.SubjectRef {
	if in == nil {
		return grantbiz.SubjectRef{}
	}
	return grantbiz.SubjectRef{Type: in.GetType(), ID: in.GetId(), Relation: in.GetRelation()}
}
func resourceRef(in *resourcev1.ResourceRef) resourcebiz.ResourceRef {
	if in == nil {
		return resourcebiz.ResourceRef{}
	}
	return resourcebiz.ResourceRef{Type: in.GetType(), ID: in.GetId()}
}
func grantResource(in *resourcev1.ResourceRef) grantbiz.ResourceRef {
	if in == nil {
		return grantbiz.ResourceRef{}
	}
	return grantbiz.ResourceRef{Type: in.GetType(), ID: in.GetId()}
}

func currentPrincipalSubject(ctx context.Context) (string, string, error) {
		principal, ok := authn.PrincipalFromContext(ctx)
		if !ok || !principal.IsAuthenticated() {
			return "", "", authn.ErrMissingCredential("kernel principal is required")
		}
		subjectType := strings.TrimSpace(principal.SubjectType)
		if subjectType == "" {
			subjectType = authn.SubjectTypeUser
		}
		return subjectType, strings.TrimSpace(principal.SubjectID), nil
	}

func currentOrgID(ctx context.Context) (string, error) {
		principal, ok := authn.PrincipalFromContext(ctx)
		if !ok || !principal.IsAuthenticated() {
			return "", authn.ErrMissingCredential("kernel principal is required")
		}
		orgID := strings.TrimSpace(principal.OrgID)
		if orgID == "" {
			return "", authn.ErrMissingCredential("kernel principal org_id is required")
		}
		return orgID, nil
	}

func currentProjectContext(ctx context.Context, pathOrgIDs ...string) (string, projectbiz.SubjectRef, error) {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || !principal.IsAuthenticated() {
		return "", projectbiz.SubjectRef{}, authn.ErrMissingCredential("kernel principal is required")
	}
	orgID := strings.TrimSpace(principal.OrgID)
	if orgID == "" {
		return "", projectbiz.SubjectRef{}, authn.ErrMissingCredential("kernel principal org_id is required")
	}
	// If a path org_id is provided, validate it matches the principal's org_id.
	if len(pathOrgIDs) > 0 && pathOrgIDs[0] != "" && !strings.EqualFold(pathOrgIDs[0], orgID) {
		return "", projectbiz.SubjectRef{}, authz.ErrPermissionDenied("org_id mismatch: path org_id does not match principal org_id")
	}
	subjectType := strings.TrimSpace(principal.SubjectType)
	if subjectType == "" {
		subjectType = authn.SubjectTypeUser
	}
	return orgID, projectbiz.SubjectRef{Type: subjectType, ID: strings.TrimSpace(principal.SubjectID)}, nil
}

func currentProjectSubject(ctx context.Context) (projectbiz.SubjectRef, error) {
	_, subject, err := currentProjectContext(ctx)
	return subject, err
}

func currentResourceSubject(ctx context.Context) (resourcebiz.SubjectRef, error) {
	subjectType, subjectID, err := currentPrincipalSubject(ctx)
	if err != nil {
		return resourcebiz.SubjectRef{}, err
	}
	return resourcebiz.SubjectRef{Type: subjectType, ID: subjectID}, nil
}

func currentGrantSubject(ctx context.Context) (grantbiz.SubjectRef, error) {
	subjectType, subjectID, err := currentPrincipalSubject(ctx)
	if err != nil {
		return grantbiz.SubjectRef{}, err
	}
	return grantbiz.SubjectRef{Type: subjectType, ID: subjectID}, nil
}

func resourceSubjectOr(in *resourcev1.SubjectRef, fallback resourcebiz.SubjectRef) resourcebiz.SubjectRef {
	subject := resourceSubject(in)
	if strings.TrimSpace(subject.Type) == "" || strings.TrimSpace(subject.ID) == "" {
		return fallback
	}
	return subject
}

func structToJSON(in *structpb.Struct, fallback string) string {
	if in == nil {
		return fallback
	}
	body, err := protojson.Marshal(in)
	if err != nil {
		return fallback
	}
	return string(body)
}
func jsonToStruct(raw string) *structpb.Struct {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out structpb.Struct
	if err := protojson.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return &out
}
func mapStringToJSON(in map[string]string) string {
	if len(in) == 0 {
		return "{}"
	}
	body, _ := json.Marshal(in)
	return string(body)
}
func stringSliceToJSON(in []string) string {
	if len(in) == 0 {
		return "[]"
	}
	body, _ := json.Marshal(in)
	return string(body)
}
func jsonToStringMap(raw string) map[string]string {
	var out map[string]string
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		return map[string]string{}
	}
	return out
}
func jsonToStringSlice(raw string) []string {
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}
func ts(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
func tsPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}
func timestampPtr(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}
func pageFromToken(v string) int {
	n, _ := strconv.Atoi(v)
	if n <= 0 {
		return 1
	}
	return n
}
func nextPage[T any](p *data.Page[T]) string {
	if p != nil && p.HasMore {
		return strconv.Itoa(p.Page + 1)
	}
	return ""
}

func lifecycleToStatus(v projectv1.LifecycleStatus) string {
	switch v {
	case projectv1.LifecycleStatus_ACTIVE:
		return data.StatusActive
	case projectv1.LifecycleStatus_ARCHIVED:
		return data.StatusArchived
	case projectv1.LifecycleStatus_DELETED:
		return data.StatusDeleted
	case projectv1.LifecycleStatus_DISABLED:
		return "disabled"
	default:
		return ""
	}
}
func statusToLifecycle(v string) projectv1.LifecycleStatus {
	switch strings.TrimSpace(v) {
	case data.StatusActive:
		return projectv1.LifecycleStatus_ACTIVE
	case data.StatusArchived:
		return projectv1.LifecycleStatus_ARCHIVED
	case data.StatusDeleted:
		return projectv1.LifecycleStatus_DELETED
	case "disabled":
		return projectv1.LifecycleStatus_DISABLED
	default:
		return projectv1.LifecycleStatus_LIFECYCLE_STATUS_UNSPECIFIED
	}
}
func visibilityToStatus(v projectv1.ProjectVisibility) string {
	switch v {
	case projectv1.ProjectVisibility_ORG:
		return "org"
	case projectv1.ProjectVisibility_PUBLIC:
		return "public"
	case projectv1.ProjectVisibility_PRIVATE:
		return "private"
	default:
		return ""
	}
}
func statusToVisibility(v string) projectv1.ProjectVisibility {
	switch strings.TrimSpace(v) {
	case "org":
		return projectv1.ProjectVisibility_ORG
	case "public":
		return projectv1.ProjectVisibility_PUBLIC
	case "private":
		return projectv1.ProjectVisibility_PRIVATE
	default:
		return projectv1.ProjectVisibility_PROJECT_VISIBILITY_UNSPECIFIED
	}
}
