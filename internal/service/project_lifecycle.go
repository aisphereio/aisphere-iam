package service

import (
	"context"
	"strings"

	projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	projectbiz "github.com/aisphereio/aisphere-iam/internal/biz/project"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *ProjectService) GetProject(ctx context.Context, req *projectv1.GetProjectRequest) (*projectv1.Project, error) {
	zoneID, _, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	project, err := s.biz.GetProject(ctx, req.GetProjectId(), zoneID)
	if err != nil {
		return nil, err
	}
	return projectModelToProto(project), nil
}

func (s *ProjectService) UpdateProject(ctx context.Context, req *projectv1.UpdateProjectRequest) (*projectv1.Project, error) {
	zoneID, _, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	update, err := projectUpdateFromProto(req, zoneID)
	if err != nil {
		return nil, err
	}
	project, err := s.biz.UpdateProject(ctx, update)
	if err != nil {
		return nil, err
	}
	return projectModelToProto(project), nil
}

func (s *ProjectService) ArchiveProject(ctx context.Context, req *projectv1.ArchiveProjectRequest) (*projectv1.Project, error) {
	zoneID, _, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	project, err := s.biz.ArchiveProject(ctx, projectbiz.ArchiveProjectRequest{
		ID: req.GetProjectId(), ZoneID: zoneID, Reason: strings.TrimSpace(req.GetReason()),
	})
	if err != nil {
		return nil, err
	}
	return projectModelToProto(project), nil
}

func (s *ProjectService) EnableProjectCapability(ctx context.Context, req *projectv1.EnableProjectCapabilityRequest) (*projectv1.ProjectCapability, error) {
	zoneID, _, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	capability, err := s.biz.SetProjectCapability(ctx, projectbiz.SetProjectCapabilityRequest{
		ZoneID: zoneID, ProjectID: req.GetProjectId(), CapabilityID: req.GetCapabilityId(), Enabled: true,
		ConfigJSON: structToJSON(req.GetConfig(), "{}"), QuotaJSON: structToJSON(req.GetQuota(), "{}"),
	})
	if err != nil {
		return nil, err
	}
	return projectCapabilityModelToProto(capability), nil
}

func (s *ProjectService) DisableProjectCapability(ctx context.Context, req *projectv1.DisableProjectCapabilityRequest) (*projectv1.ProjectCapability, error) {
	zoneID, _, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	capability, err := s.biz.SetProjectCapability(ctx, projectbiz.SetProjectCapabilityRequest{
		ZoneID: zoneID, ProjectID: req.GetProjectId(), CapabilityID: req.GetCapabilityId(), Enabled: false,
	})
	if err != nil {
		return nil, err
	}
	return projectCapabilityModelToProto(capability), nil
}

func (s *ProjectService) ListProjectCapabilities(ctx context.Context, req *projectv1.ListProjectCapabilitiesRequest) (*projectv1.ListProjectCapabilitiesReply, error) {
	zoneID, _, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.biz.GetProject(ctx, req.GetProjectId(), zoneID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListProjectCapabilities(ctx, req.GetProjectId())
	if err != nil {
		return nil, err
	}
	out := make([]*projectv1.ProjectCapability, 0, len(items))
	for i := range items {
		if req.Enabled != nil && items[i].Enabled != req.GetEnabled() {
			continue
		}
		out = append(out, projectCapabilityModelToProto(&items[i]))
	}
	return &projectv1.ListProjectCapabilitiesReply{Capabilities: out}, nil
}

func projectUpdateFromProto(req *projectv1.UpdateProjectRequest, zoneID string) (projectbiz.UpdateProjectRequest, error) {
	out := projectbiz.UpdateProjectRequest{ID: req.GetProjectId(), ZoneID: zoneID}
	paths := req.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		if req.GetDisplayName() != "" {
			value := req.GetDisplayName()
			out.DisplayName = &value
		}
		if req.GetDescription() != "" {
			value := req.GetDescription()
			out.Description = &value
		}
		if value := visibilityToStatus(req.GetVisibility()); value != "" {
			out.Visibility = &value
		}
		if req.Labels != nil {
			value := mapStringToJSON(req.GetLabels())
			out.LabelsJSON = &value
		}
		if req.Annotations != nil {
			value := mapStringToJSON(req.GetAnnotations())
			out.AnnotationsJSON = &value
		}
		if req.GetMetadata() != nil {
			value := structToJSON(req.GetMetadata(), "{}")
			out.MetadataJSON = &value
		}
		return out, nil
	}

	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		switch path {
		case "display_name":
			value := req.GetDisplayName()
			out.DisplayName = &value
		case "description":
			value := req.GetDescription()
			out.Description = &value
		case "visibility":
			value := visibilityToStatus(req.GetVisibility())
			if value == "" {
				return out, status.Error(codes.InvalidArgument, "visibility must be PRIVATE, ORG or PUBLIC")
			}
			out.Visibility = &value
		case "labels":
			value := mapStringToJSON(req.GetLabels())
			out.LabelsJSON = &value
		case "annotations":
			value := mapStringToJSON(req.GetAnnotations())
			out.AnnotationsJSON = &value
		case "metadata":
			value := structToJSON(req.GetMetadata(), "{}")
			out.MetadataJSON = &value
		default:
			return out, status.Errorf(codes.InvalidArgument, "unsupported update_mask path %q", path)
		}
	}
	return out, nil
}

func projectSubjectStringToProto(raw string) *resourcev1.SubjectRef {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	relation := ""
	if index := strings.LastIndex(raw, "#"); index >= 0 {
		relation = raw[index+1:]
		raw = raw[:index]
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return nil
	}
	return &resourcev1.SubjectRef{Type: strings.TrimSpace(parts[0]), Id: strings.TrimSpace(parts[1]), Relation: strings.TrimSpace(relation)}
}

var _ = data.StatusActive
