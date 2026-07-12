// Package project owns Aisphere project/capability control-plane use cases.
//
// Casdoor Organization is the single organization root and is represented in
// authorization as a zone. The legacy IAM-local Organization methods remain
// temporarily for source compatibility while callers migrate; new project
// relationships must never use that legacy resource type.
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
	// ResourceTypeOrganization is retained only for the legacy control-plane
	// Organization API. It must not be used by new project relationships.
	ResourceTypeOrganization = "organization"
	ResourceTypeZone         = "zone"
	ResourceTypeProject      = "project"

	RelationParent = "parent"
	RelationZone   = "zone"
	RelationOwner  = "owner"
	RelationAdmin  = "admin"
	RelationMember = "member"
)

type SubjectRef struct {
	Type     string
	ID       string
	Relation string
}

type CreateOrganizationRequest struct {
	ID           string
	Slug         string
	DisplayName  string
	CasdoorOrg   string
	Plan         string
	Region       string
	MetadataJSON string
	Owner        SubjectRef
}

type CreateProjectRequest struct {
	ID string

	// ZoneID is the canonical Casdoor organization identifier. OrgID is a
	// temporary compatibility input and must be removed with the legacy proto.
	ZoneID string
	OrgID  string

	Slug            string
	DisplayName     string
	Description     string
	Visibility      string
	LabelsJSON      string
	AnnotationsJSON string
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
	ProjectID    string
	CapabilityID string
	Enabled      bool
	ConfigJSON   string
	QuotaJSON    string
}

type Service struct {
	repo       data.ControlPlaneRepository
	projection *projection.Manager
	now        func() time.Time
}

func NewService(repo data.ControlPlaneRepository, writer authz.RelationshipWriter, managers ...*projection.Manager) *Service {
	pm := firstProjectionManager(managers)
	if pm == nil {
		pm = projection.NewManager(repo, writer, nil)
	}
	return &Service{repo: repo, projection: pm, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) CreateOrganization(ctx context.Context, req CreateOrganizationRequest) (*data.OrganizationModel, authz.WriteResult, error) {
	if s.repo == nil {
		return nil, authz.WriteResult{}, errors.New("project service repository is nil")
	}
	req.Slug = strings.TrimSpace(req.Slug)
	if req.Slug == "" {
		return nil, authz.WriteResult{}, errors.New("organization slug is required")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = idgen.New("org")
	}
	now := s.now()
	org := &data.OrganizationModel{
		ID:           id,
		Slug:         req.Slug,
		DisplayName:  nonEmpty(req.DisplayName, req.Slug),
		Status:       data.StatusActive,
		CasdoorOrg:   strings.TrimSpace(req.CasdoorOrg),
		Plan:         strings.TrimSpace(req.Plan),
		Region:       strings.TrimSpace(req.Region),
		MetadataJSON: jsonOrEmptyObject(req.MetadataJSON),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	rels := make([]authz.Relationship, 0, 3)
	if !subjectZero(req.Owner) {
		ownerSubj := toAuthzSubject(req.Owner)
		rels = append(rels,
			authz.Relationship{
				Resource: graph.Object(ResourceTypeOrganization, org.ID),
				Relation: RelationOwner,
				Subject:  ownerSubj,
			},
			authz.Relationship{
				Resource: graph.Object(ResourceTypeOrganization, org.ID),
				Relation: RelationAdmin,
				Subject:  ownerSubj,
			},
			authz.Relationship{
				Resource: graph.Object(ResourceTypeOrganization, org.ID),
				Relation: RelationMember,
				Subject:  ownerSubj,
			},
		)
	}
	event, err := s.projection.NewWriteEvent("organization", org.ID, rels...)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if err := s.repo.CreateOrganization(ctx, org, event); err != nil {
		return nil, authz.WriteResult{}, err
	}
	wr, err := s.projection.Dispatch(ctx, event)
	return org, wr, err
}

type UpdateOrganizationRequest struct {
	ID           string
	DisplayName  string
	Plan         string
	Region       string
	MetadataJSON string
}

func (s *Service) UpdateOrganization(ctx context.Context, req UpdateOrganizationRequest) (*data.OrganizationModel, error) {
	if s.repo == nil {
		return nil, errors.New("project service repository is nil")
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		return nil, errors.New("organization id is required")
	}
	org, err := s.repo.GetOrganization(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.DisplayName != "" {
		org.DisplayName = strings.TrimSpace(req.DisplayName)
	}
	if req.Plan != "" {
		org.Plan = strings.TrimSpace(req.Plan)
	}
	if req.Region != "" {
		org.Region = strings.TrimSpace(req.Region)
	}
	if req.MetadataJSON != "" {
		org.MetadataJSON = req.MetadataJSON
	}
	org.UpdatedAt = s.now()
	if err := s.repo.UpsertOrganization(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *Service) ArchiveOrganization(ctx context.Context, id string) (*data.OrganizationModel, error) {
	if s.repo == nil {
		return nil, errors.New("project service repository is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("organization id is required")
	}
	org, err := s.repo.GetOrganization(ctx, id)
	if err != nil {
		return nil, err
	}
	if org.Status == data.StatusArchived {
		return org, nil
	}
	org.Status = data.StatusArchived
	org.UpdatedAt = s.now()
	if err := s.repo.UpsertOrganization(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *Service) CreateProject(ctx context.Context, req CreateProjectRequest) (*data.ProjectModel, authz.WriteResult, error) {
	if s.repo == nil {
		return nil, authz.WriteResult{}, errors.New("project service repository is nil")
	}
	zoneID := strings.TrimSpace(req.ZoneID)
	if zoneID == "" {
		zoneID = strings.TrimSpace(req.OrgID)
	}
	req.Slug = strings.TrimSpace(req.Slug)
	if zoneID == "" {
		return nil, authz.WriteResult{}, errors.New("zone_id is required")
	}
	if req.Slug == "" {
		return nil, authz.WriteResult{}, errors.New("project slug is required")
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
		CreatedBy:       subjectString(req.CreatedBy),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	rels := []authz.Relationship{{
		Resource: graph.Object(ResourceTypeProject, project.ID),
		Relation: RelationZone,
		Subject:  graph.Subject(ResourceTypeZone, zoneID, ""),
	}}
	if !subjectZero(req.Owner) {
		rels = append(rels, authz.Relationship{
			Resource: graph.Object(ResourceTypeProject, project.ID),
			Relation: RelationOwner,
			Subject:  toAuthzSubject(req.Owner),
		})
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
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.CapabilityID = strings.TrimSpace(req.CapabilityID)
	if req.ProjectID == "" || req.CapabilityID == "" {
		return nil, errors.New("project_id and capability_id are required")
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
