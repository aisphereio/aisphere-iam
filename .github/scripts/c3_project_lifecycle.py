from pathlib import Path
import re
import textwrap


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing expected fragment: {label}")
    return text.replace(old, new, 1)

# Proto: add PATCH field presence through FieldMask.
proto_path = Path("api/iam/project/v1/project.proto")
proto = proto_path.read_text()
proto = replace_once(
    proto,
    'import "google/protobuf/field_behavior.proto";\n',
    'import "google/protobuf/field_behavior.proto";\nimport "google/protobuf/field_mask.proto";\n',
    "project field mask import",
)
proto = replace_once(
    proto,
    '  google.protobuf.Struct metadata = 7;\n}',
    '  google.protobuf.Struct metadata = 7;\n  google.protobuf.FieldMask update_mask = 8;\n}',
    "UpdateProjectRequest update_mask",
)
proto_path.write_text(proto)

# Project data model: metadata is part of the public contract and must persist.
models_path = Path("internal/data/resource_models.go")
models = models_path.read_text()
models = replace_once(
    models,
    '\tAnnotationsJSON string     `gorm:"column:annotations_json;type:jsonb;default:\'{}\'" json:"annotations_json"`\n\tCreatedBy',
    '\tAnnotationsJSON string     `gorm:"column:annotations_json;type:jsonb;default:\'{}\'" json:"annotations_json"`\n\tMetadataJSON    string     `gorm:"column:metadata_json;type:jsonb;default:\'{}\'" json:"metadata_json"`\n\tCreatedBy',
    "ProjectModel metadata",
)
models_path.write_text(models)

# DB repository persists metadata on updates.
repo_path = Path("internal/data/resource_repository.go")
repo = repo_path.read_text()
repo = replace_once(
    repo,
    '[]string{"display_name", "description", "status", "visibility", "labels_json", "annotations_json", "updated_at"}',
    '[]string{"display_name", "description", "status", "visibility", "labels_json", "annotations_json", "metadata_json", "updated_at"}',
    "project upsert columns",
)
repo_path.write_text(repo)

# Create path and capability mutation business inputs.
project_path = Path("internal/biz/project/project.go")
project = project_path.read_text()
project = replace_once(
    project,
    '\tAnnotationsJSON string\n\tCreatedBy',
    '\tAnnotationsJSON string\n\tMetadataJSON    string\n\tCreatedBy',
    "CreateProjectRequest metadata",
)
project = replace_once(
    project,
    'type SetProjectCapabilityRequest struct {\n\tProjectID',
    'type SetProjectCapabilityRequest struct {\n\tZoneID      string\n\tProjectID',
    "SetProjectCapabilityRequest zone",
)
project = replace_once(
    project,
    '\t\tAnnotationsJSON: jsonOrEmptyObject(req.AnnotationsJSON),\n\t\tCreatedBy:',
    '\t\tAnnotationsJSON: jsonOrEmptyObject(req.AnnotationsJSON),\n\t\tMetadataJSON:    jsonOrEmptyObject(req.MetadataJSON),\n\t\tCreatedBy:',
    "ProjectModel create metadata",
)
project = replace_once(
    project,
    '\treq.ProjectID = strings.TrimSpace(req.ProjectID)\n\treq.CapabilityID = strings.TrimSpace(req.CapabilityID)',
    '\treq.ZoneID = strings.TrimSpace(req.ZoneID)\n\treq.ProjectID = strings.TrimSpace(req.ProjectID)\n\treq.CapabilityID = strings.TrimSpace(req.CapabilityID)',
    "capability zone normalization",
)
project = replace_once(
    project,
    '\tif req.ProjectID == "" || req.CapabilityID == "" {\n\t\treturn nil, errors.New("project_id and capability_id are required")\n\t}\n\tnow := s.now()',
    '\tif req.ZoneID == "" || req.ProjectID == "" || req.CapabilityID == "" {\n\t\treturn nil, errors.New("zone_id, project_id and capability_id are required")\n\t}\n\tproject, err := s.loadProjectInZone(ctx, req.ProjectID, req.ZoneID)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tif project.Status != data.StatusActive {\n\t\treturn nil, errors.New("project is not active")\n\t}\n\tnow := s.now()',
    "capability active project check",
)
project_path.write_text(project)

# Remove old Project service methods that returned Unimplemented or bypassed Zone lifecycle checks.
service_path = Path("internal/service/control_plane.go")
service = service_path.read_text()
method_names = [
    "GetProject",
    "UpdateProject",
    "ArchiveProject",
    "EnableProjectCapability",
    "DisableProjectCapability",
    "ListProjectCapabilities",
]
for name in method_names:
    pattern = re.compile(
        rf'func \(s \*ProjectService\) {name}\([^\n]*\n(?:.*\n)*?\}}\n\n',
        re.MULTILINE,
    )
    service, count = pattern.subn("", service, count=1)
    if count != 1:
        raise SystemExit(f"failed to remove ProjectService.{name}: {count}")

service = replace_once(
    service,
    'Visibility: visibilityToStatus(req.GetVisibility()), LabelsJSON: mapStringToJSON(req.GetLabels()), AnnotationsJSON: mapStringToJSON(req.GetAnnotations()),\n\t\tCreatedBy:',
    'Visibility: visibilityToStatus(req.GetVisibility()), LabelsJSON: mapStringToJSON(req.GetLabels()), AnnotationsJSON: mapStringToJSON(req.GetAnnotations()), MetadataJSON: structToJSON(req.GetMetadata(), "{}"),\n\t\tCreatedBy:',
    "CreateProject service metadata",
)
service = replace_once(
    service,
    'Annotations: jsonToStringMap(in.AnnotationsJSON), CreatedAt:',
    'Annotations: jsonToStringMap(in.AnnotationsJSON), Metadata: jsonToStruct(in.MetadataJSON), CreatedBy: projectSubjectStringToProto(in.CreatedBy), CreatedAt:',
    "Project response metadata and creator",
)
service_path.write_text(service)

# Memory repository must fully implement ControlPlaneRepository project/outbox signatures.
memory_path = Path("internal/data/memory.go")
memory = memory_path.read_text()
memory = replace_once(
    memory,
    'func (r *MemoryControlPlaneRepository) CreateProject(ctx context.Context, p *ProjectModel, event *OutboxEventModel) error {',
    'func (r *MemoryControlPlaneRepository) CreateProject(ctx context.Context, p *ProjectModel, events ...*OutboxEventModel) error {',
    "memory CreateProject variadic",
)
memory = replace_once(
    memory,
    '\tr.saveEvent(event)\n\treturn nil\n}\nfunc (r *MemoryControlPlaneRepository) GetProject',
    '\tfor _, event := range events {\n\t\tr.saveEvent(event)\n\t}\n\treturn nil\n}\nfunc (r *MemoryControlPlaneRepository) GetProject',
    "memory project outbox loop",
)
memory = replace_once(
    memory,
    'func (r *MemoryControlPlaneRepository) UpsertResource(ctx context.Context, res *ResourceModel, event *OutboxEventModel) error {',
    'func (r *MemoryControlPlaneRepository) UpsertResource(ctx context.Context, res *ResourceModel, events ...*OutboxEventModel) error {',
    "memory UpsertResource variadic",
)
# Replace the next matching resource saveEvent only after UpsertResource.
resource_start = memory.index('func (r *MemoryControlPlaneRepository) UpsertResource')
resource_tail = memory[resource_start:]
resource_tail = replace_once(
    resource_tail,
    '\tr.saveEvent(event)\n\treturn nil\n}',
    '\tfor _, event := range events {\n\t\tr.saveEvent(event)\n\t}\n\treturn nil\n}',
    "memory resource outbox loop",
)
memory = memory[:resource_start] + resource_tail
memory = memory.replace(
    'func (r *MemoryControlPlaneRepository) BindResource(ctx context.Context, b *ResourceBindingModel, event *OutboxEventModel) error {',
    'func (r *MemoryControlPlaneRepository) BindResource(ctx context.Context, b *ResourceBindingModel, events ...*OutboxEventModel) error {',
    1,
)
if 'func (r *MemoryControlPlaneRepository) BindResource(ctx context.Context, b *ResourceBindingModel, events ...*OutboxEventModel) error {' not in memory:
    raise SystemExit("missing memory BindResource signature")
binding_start = memory.index('func (r *MemoryControlPlaneRepository) BindResource')
binding_tail = memory[binding_start:]
binding_tail = replace_once(
    binding_tail,
    '\tr.saveEvent(event)\n\treturn nil\n}',
    '\tfor _, event := range events {\n\t\tr.saveEvent(event)\n\t}\n\treturn nil\n}',
    "memory binding outbox loop",
)
memory = memory[:binding_start] + binding_tail
memory_path.write_text(memory)

Path("internal/data/project_lifecycle.go").write_text(textwrap.dedent("""\
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
"""))

Path("internal/biz/project/lifecycle.go").write_text(textwrap.dedent("""\
    package project

    import (
        "context"
        "encoding/json"
        "errors"
        "strings"

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
        return project, nil
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
"""))

Path("internal/service/project_lifecycle.go").write_text(textwrap.dedent("""\
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
            if req.GetDisplayName() != "" { value := req.GetDisplayName(); out.DisplayName = &value }
            if req.GetDescription() != "" { value := req.GetDescription(); out.Description = &value }
            if value := visibilityToStatus(req.GetVisibility()); value != "" { out.Visibility = &value }
            if req.Labels != nil { value := mapStringToJSON(req.GetLabels()); out.LabelsJSON = &value }
            if req.Annotations != nil { value := mapStringToJSON(req.GetAnnotations()); out.AnnotationsJSON = &value }
            if req.GetMetadata() != nil { value := structToJSON(req.GetMetadata(), "{}"); out.MetadataJSON = &value }
            return out, nil
        }

        seen := make(map[string]struct{}, len(paths))
        for _, raw := range paths {
            path := strings.TrimSpace(raw)
            if _, ok := seen[path]; ok { continue }
            seen[path] = struct{}{}
            switch path {
            case "display_name":
                value := req.GetDisplayName(); out.DisplayName = &value
            case "description":
                value := req.GetDescription(); out.Description = &value
            case "visibility":
                value := visibilityToStatus(req.GetVisibility())
                if value == "" { return out, status.Error(codes.InvalidArgument, "visibility must be PRIVATE, ORG or PUBLIC") }
                out.Visibility = &value
            case "labels":
                value := mapStringToJSON(req.GetLabels()); out.LabelsJSON = &value
            case "annotations":
                value := mapStringToJSON(req.GetAnnotations()); out.AnnotationsJSON = &value
            case "metadata":
                value := structToJSON(req.GetMetadata(), "{}"); out.MetadataJSON = &value
            default:
                return out, status.Errorf(codes.InvalidArgument, "unsupported update_mask path %q", path)
            }
        }
        return out, nil
    }

    func projectSubjectStringToProto(raw string) *resourcev1.SubjectRef {
        raw = strings.TrimSpace(raw)
        if raw == "" { return nil }
        relation := ""
        if index := strings.LastIndex(raw, "#"); index >= 0 {
            relation = raw[index+1:]
            raw = raw[:index]
        }
        parts := strings.SplitN(raw, ":", 2)
        if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" { return nil }
        return &resourcev1.SubjectRef{Type: strings.TrimSpace(parts[0]), Id: strings.TrimSpace(parts[1]), Relation: strings.TrimSpace(relation)}
    }

    var _ = data.StatusActive
"""))

Path("internal/biz/project/lifecycle_test.go").write_text(textwrap.dedent("""\
    package project

    import (
        "context"
        "testing"
        "time"

        "github.com/aisphereio/aisphere-iam/internal/data"
        "github.com/aisphereio/kernel/authz"
    )

    func TestProjectLifecyclePreservesIdentityAndBlocksArchivedMutation(t *testing.T) {
        ctx := context.Background()
        repo := data.NewMemoryControlPlaneRepository()
        service := NewService(repo, authz.NewMemoryRelationshipStore())
        fixed := time.Date(2026, 7, 13, 5, 0, 0, 0, time.UTC)
        service.now = func() time.Time { return fixed }

        project, _, err := service.CreateProject(ctx, CreateProjectRequest{
            ID: "project-1", ZoneID: "zone-a", Slug: "alpha", DisplayName: "Alpha",
            MetadataJSON: `{"tier":"dev"}`,
            CreatedBy: SubjectRef{Type: "user", ID: "alice"},
            Owner: SubjectRef{Type: "user", ID: "alice"},
        })
        if err != nil { t.Fatalf("CreateProject: %v", err) }

        display := "Alpha Updated"
        description := ""
        visibility := "org"
        labels := `{"team":"platform"}`
        metadata := `{"tier":"prod"}`
        updated, err := service.UpdateProject(ctx, UpdateProjectRequest{
            ID: project.ID, ZoneID: "zone-a", DisplayName: &display, Description: &description,
            Visibility: &visibility, LabelsJSON: &labels, MetadataJSON: &metadata,
        })
        if err != nil { t.Fatalf("UpdateProject: %v", err) }
        if updated.OrgID != "zone-a" || updated.Slug != "alpha" || updated.CreatedBy != "user:alice" {
            t.Fatalf("immutable project identity changed: %#v", updated)
        }
        if updated.DisplayName != display || updated.Description != "" || updated.Visibility != "org" || updated.MetadataJSON != `{"tier":"prod"}` {
            t.Fatalf("project fields not updated: %#v", updated)
        }

        archived, err := service.ArchiveProject(ctx, ArchiveProjectRequest{ID: project.ID, ZoneID: "zone-a"})
        if err != nil { t.Fatalf("ArchiveProject: %v", err) }
        if archived.Status != data.StatusArchived { t.Fatalf("status = %q", archived.Status) }
        again, err := service.ArchiveProject(ctx, ArchiveProjectRequest{ID: project.ID, ZoneID: "zone-a"})
        if err != nil || again.Status != data.StatusArchived { t.Fatalf("idempotent archive = (%#v, %v)", again, err) }
        if _, err := service.UpdateProject(ctx, UpdateProjectRequest{ID: project.ID, ZoneID: "zone-a", DisplayName: &display}); err == nil {
            t.Fatal("expected archived project update to fail")
        }
        if _, err := service.SetProjectCapability(ctx, SetProjectCapabilityRequest{ZoneID: "zone-a", ProjectID: project.ID, CapabilityID: "skills", Enabled: true}); err == nil {
            t.Fatal("expected archived project capability mutation to fail")
        }
        if _, err := service.GetProject(ctx, project.ID, "zone-b"); err == nil {
            t.Fatal("expected cross-zone read to fail")
        }
    }
"""))

Path("internal/service/project_lifecycle_test.go").write_text(textwrap.dedent("""\
    package service

    import (
        "context"
        "testing"

        projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
        projectbiz "github.com/aisphereio/aisphere-iam/internal/biz/project"
        "github.com/aisphereio/aisphere-iam/internal/data"
        "github.com/aisphereio/kernel/authn"
        "github.com/aisphereio/kernel/authz"
        "google.golang.org/protobuf/types/known/fieldmaskpb"
        "google.golang.org/protobuf/types/known/structpb"
    )

    func TestProjectServiceUpdateMaskAndZoneScope(t *testing.T) {
        repo := data.NewMemoryControlPlaneRepository()
        biz := projectbiz.NewService(repo, authz.NewMemoryRelationshipStore())
        service := NewProjectService(biz, repo)
        ctx := authn.ContextWithPrincipal(context.Background(), authn.Principal{SubjectID: "alice", SubjectType: "user", OrgID: "zone-a"})

        created, err := service.CreateProject(ctx, &projectv1.CreateProjectRequest{Slug: "alpha", DisplayName: "Alpha"})
        if err != nil { t.Fatalf("CreateProject: %v", err) }
        metadata, _ := structpb.NewStruct(map[string]any{"tier": "prod"})
        updated, err := service.UpdateProject(ctx, &projectv1.UpdateProjectRequest{
            ProjectId: created.GetId(), Description: "", Metadata: metadata,
            UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"description", "metadata"}},
        })
        if err != nil { t.Fatalf("UpdateProject: %v", err) }
        if updated.GetDescription() != "" || updated.GetMetadata().GetFields()["tier"].GetStringValue() != "prod" {
            t.Fatalf("unexpected update response: %#v", updated)
        }
        if updated.GetCreatedBy().GetId() != "alice" {
            t.Fatalf("created_by was not returned: %#v", updated.GetCreatedBy())
        }

        other := authn.ContextWithPrincipal(context.Background(), authn.Principal{SubjectID: "bob", SubjectType: "user", OrgID: "zone-b"})
        if _, err := service.GetProject(other, &projectv1.GetProjectRequest{ProjectId: created.GetId()}); err == nil {
            t.Fatal("expected cross-zone project read to fail")
        }
        if _, err := service.UpdateProject(ctx, &projectv1.UpdateProjectRequest{ProjectId: created.GetId(), UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"slug"}}}); err == nil {
            t.Fatal("expected immutable/unknown update path to fail")
        }
    }
"""))

Path("internal/service/project_lifecycle_contract_test.go").write_text(textwrap.dedent("""\
    package service

    import (
        "os"
        "path/filepath"
        "strings"
        "testing"
    )

    func TestProjectLifecycleContractHasNoUnimplementedHandlers(t *testing.T) {
        root := filepath.Join("..", "..")
        source, err := os.ReadFile(filepath.Join(root, "internal", "service", "control_plane.go"))
        if err != nil { t.Fatal(err) }
        text := string(source)
        for _, forbidden := range []string{"UpdateProject is not implemented", "ArchiveProject is not implemented"} {
            if strings.Contains(text, forbidden) { t.Fatalf("project lifecycle stub returned: %s", forbidden) }
        }
        proto, err := os.ReadFile(filepath.Join(root, "api", "iam", "project", "v1", "project.proto"))
        if err != nil { t.Fatal(err) }
        if !strings.Contains(string(proto), "google.protobuf.FieldMask update_mask") {
            t.Fatal("UpdateProjectRequest must define update_mask")
        }
    }
"""))

Path(".agile-v/change_requests/CR-0003-complete-project-lifecycle.md").write_text(textwrap.dedent("""\
    # CR-0003 — Complete Project Lifecycle

    ## Status

    `IMPLEMENTED_PENDING_VERIFICATION [C3]`

    ## Scope

    - implement Project update and archive operations that were previously exposed but returned `Unimplemented`;
    - use `FieldMask` for explicit PATCH semantics, including field clearing;
    - persist and return Project metadata;
    - preserve immutable Project identity, Zone, slug, creator and ownership projection;
    - enforce Principal Zone scope on Project reads and Project Capability operations;
    - reject updates and capability mutations after archive;
    - make archive idempotent;
    - add business, service and contract regression tests.

    ## Mutable fields

    - `display_name`
    - `description`
    - `visibility`
    - `labels`
    - `annotations`
    - `metadata`

    `org_id`, `slug`, `created_by`, owner relationships and lifecycle timestamps are not client-mutable through UpdateProject.

    ## Acceptance criteria

    - `UpdateProject` and `ArchiveProject` no longer return `Unimplemented`;
    - omitted update mask preserves backward-compatible non-zero-field patch behavior;
    - explicit mask supports clearing description/maps/metadata;
    - unknown or immutable mask paths are rejected;
    - cross-Zone Project access is rejected;
    - archived Project update and capability mutation are rejected;
    - archive is idempotent;
    - generation, contract checks, all Go tests, build and generated drift checks pass.
"""))
