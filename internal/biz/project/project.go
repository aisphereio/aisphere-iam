// Package project owns Aisphere project/capability control-plane use cases.
//
// Casdoor Organization is the single identity-domain root and is represented
// inside Aisphere authorization as zone:<principal.org_id>. IAM does not create
// or persist a second platform Organization model.
package project

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authz"

	"github.com/aisphereio/aisphere-iam/internal/biz/graph"
	"github.com/aisphereio/aisphere-iam/internal/biz/idgen"
	"github.com/aisphereio/aisphere-iam/internal/biz/projection"
	"github.com/aisphereio/aisphere-iam/internal/data"
)

const (
	ResourceTypeZone    = "zone"
	ResourceTypeProject = "project"

	RelationZone  = "zone"
	RelationOwner = "owner"
)

type SubjectRef struct {
	Type     string
	ID       string
	Relation string
}

type CreateProjectRequest struct {
	ID string

	// ZoneID is the current authenticated Principal.org_id. Callers must not
	// supply or override this value through the public API.
	ZoneID string

	Slug            string
	DisplayName     string
	Description     string
	Visibility      string
	LabelsJSON      string
	AnnotationsJSON string
	MetadataJSON    string
	CreatedBy       SubjectRef
	Owner           SubjectRef
}

type RegisterCapabilityRequest struct {
	ID           string
	Name         string
	DisplayName  string
	OwnerService string
	ConfigSchema string
}

type SetProjectCapabilityRequest struct {
	ZoneID       string
	ProjectID    string
	CapabilityID string
	Enabled      bool
	ConfigJSON   string
	QuotaJSON    string
}

type Service struct {
	repo       data.ControlPlaneRepository
	projection *projection.Manager
	reader     authz.RelationshipReader
	now        func() time.Time
}

func NewService(repo data.ControlPlaneRepository, writer authz.RelationshipWriter, managers ...*projection.Manager) *Service {
	pm := firstProjectionManager(managers)
	if pm == nil {
		pm = projection.NewManager(repo, writer, nil)
	}
	var reader authz.RelationshipReader
	if r, ok := writer.(authz.RelationshipReader); ok {
		reader = r
	}
	return &Service{repo: repo, projection: pm, reader: reader, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) CreateProject(ctx context.Context, req CreateProjectRequest) (*data.ProjectModel, authz.WriteResult, error) {
	if s.repo == nil {
		return nil, authz.WriteResult{}, errors.New("project service repository is nil")
	}
	zoneID := strings.TrimSpace(req.ZoneID)
	req.Slug = strings.TrimSpace(req.Slug)
	if zoneID == "" {
		return nil, authz.WriteResult{}, errors.New("zone_id is required")
	}
	if req.Slug == "" {
		return nil, authz.WriteResult{}, errors.New("project slug is required")
	}
	if subjectZero(req.CreatedBy) {
		return nil, authz.WriteResult{}, errors.New("authenticated project creator is required")
	}
	if subjectZero(req.Owner) {
		return nil, authz.WriteResult{}, errors.New("project owner is required")
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = idgen.New("project")
	}
	now := s.now()
	project := &data.ProjectModel{
		ID:              id,
		OrgID:           zoneID,
		Slug:            req.Slug,
		DisplayName:     nonEmpty(req.DisplayName, req.Slug),
		Description:     strings.TrimSpace(req.Description),
		Status:          data.StatusActive,
		Visibility:      nonEmpty(req.Visibility, "private"),
		LabelsJSON:      jsonOrEmptyObject(req.LabelsJSON),
		AnnotationsJSON: jsonOrEmptyObject(req.AnnotationsJSON),
		MetadataJSON:    jsonOrEmptyObject(req.MetadataJSON),
		CreatedBy:       subjectString(req.CreatedBy),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	rels := []authz.Relationship{
		{
			Resource: graph.Object(ResourceTypeProject, project.ID),
			Relation: RelationZone,
			Subject:  graph.Subject(ResourceTypeZone, zoneID, ""),
		},
		{
			Resource: graph.Object(ResourceTypeProject, project.ID),
			Relation: RelationOwner,
			Subject:  toAuthzSubject(req.Owner),
		},
	}
	event, err := s.projection.NewWriteEvent("project", project.ID, rels...)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if err := s.repo.CreateProject(ctx, project, event); err != nil {
		return nil, authz.WriteResult{}, err
	}
	wr, err := s.projection.Dispatch(ctx, event)
	return project, wr, err
}

func (s *Service) RegisterCapability(ctx context.Context, req RegisterCapabilityRequest) (*data.CapabilityModel, error) {
	if s.repo == nil {
		return nil, errors.New("project service repository is nil")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, errors.New("capability name is required")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = req.Name
	}
	now := s.now()
	cap := &data.CapabilityModel{
		ID:           id,
		Name:         req.Name,
		DisplayName:  nonEmpty(req.DisplayName, req.Name),
		OwnerService: strings.TrimSpace(req.OwnerService),
		Status:       data.StatusActive,
		ConfigSchema: jsonOrEmptyObject(req.ConfigSchema),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if cap.OwnerService == "" {
		cap.OwnerService = "aisphere-iam"
	}
	if err := s.repo.UpsertCapability(ctx, cap); err != nil {
		return nil, err
	}
	return cap, nil
}

func (s *Service) SetProjectCapability(ctx context.Context, req SetProjectCapabilityRequest) (*data.ProjectCapabilityModel, error) {
	if s.repo == nil {
		return nil, errors.New("project service repository is nil")
	}
	req.ZoneID = strings.TrimSpace(req.ZoneID)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.CapabilityID = strings.TrimSpace(req.CapabilityID)
	if req.ZoneID == "" || req.ProjectID == "" || req.CapabilityID == "" {
		return nil, errors.New("zone_id, project_id and capability_id are required")
	}
	project, err := s.loadProjectInZone(ctx, req.ProjectID, req.ZoneID)
	if err != nil {
		return nil, err
	}
	if project.Status != data.StatusActive {
		return nil, errors.New("project is not active")
	}
	now := s.now()
	pc := &data.ProjectCapabilityModel{
		ProjectID:    req.ProjectID,
		CapabilityID: req.CapabilityID,
		Enabled:      req.Enabled,
		ConfigJSON:   jsonOrEmptyObject(req.ConfigJSON),
		QuotaJSON:    jsonOrEmptyObject(req.QuotaJSON),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.SetProjectCapability(ctx, pc); err != nil {
		return nil, err
	}
	return pc, nil
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(fallback)
}

func jsonOrEmptyObject(v string) string {
	if strings.TrimSpace(v) == "" {
		return "{}"
	}
	return strings.TrimSpace(v)
}

func subjectZero(s SubjectRef) bool {
	return strings.TrimSpace(s.Type) == "" || strings.TrimSpace(s.ID) == ""
}

func toAuthzSubject(s SubjectRef) authz.SubjectRef {
	rel := strings.TrimSpace(s.Relation)
	if strings.TrimSpace(s.Type) == authz.SubjectTypeGroup && rel == "" {
		rel = "member"
	}
	return graph.Subject(s.Type, s.ID, rel)
}

func subjectString(s SubjectRef) string {
	if subjectZero(s) {
		return ""
	}
	return toAuthzSubject(s).String()
}

func firstProjectionManager(managers []*projection.Manager) *projection.Manager {
	for _, manager := range managers {
		if manager != nil {
			return manager
		}
	}
	return nil
}
