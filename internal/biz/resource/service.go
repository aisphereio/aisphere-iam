// Package resource owns Aisphere resource-type, resource-projection and
// cross-resource binding use cases.
package resource

import (
	"context"
	"encoding/json"
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
	RelationParent = "parent"
)

type ResourceRef struct{ Type, ID string }
type SubjectRef struct{ Type, ID, Relation string }

type RegisterResourceTypeRequest struct {
	Type            string
	CapabilityID    string
	OwnerService    string
	ParentTypesJSON string
	Grantable       bool
	Auditable       bool
	SpiceDBType     string
	RelationsJSON   string
	PermissionsJSON string
	MetadataSchema  string
}

type UpsertResourceRequest struct {
	Ref             ResourceRef
	OrgID           string
	ProjectID       string
	Parent          ResourceRef
	OwnerService    string
	OwnerResourceID string
	Slug            string
	DisplayName     string
	Path            string
	Status          string
	Visibility      string
	LabelsJSON      string
	AnnotationsJSON string
	MetadataJSON    string
	CreatedBy       SubjectRef
	Owner           SubjectRef
}

type BindResourceRequest struct {
	ID        string
	OrgID     string
	Source    ResourceRef
	Relation  string
	Target    ResourceRef
	CreatedBy SubjectRef
}

type BindExternalResourceRequest struct {
	ID           string
	OrgID        string
	Resource     ResourceRef
	Provider     string
	ExternalType string
	ExternalID   string
	ExternalPath string
	ExternalURL  string
	SyncMode     string
	MetadataJSON string
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

func (s *Service) RegisterResourceType(ctx context.Context, req RegisterResourceTypeRequest) (*data.ResourceTypeModel, error) {
	if s.repo == nil {
		return nil, errors.New("resource service repository is nil")
	}
	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "" {
		return nil, errors.New("resource type is required")
	}
	if strings.TrimSpace(req.SpiceDBType) == "" {
		req.SpiceDBType = req.Type
	}
	now := s.now()
	rt := &data.ResourceTypeModel{
		Type:            req.Type,
		CapabilityID:    strings.TrimSpace(req.CapabilityID),
		OwnerService:    strings.TrimSpace(req.OwnerService),
		ParentTypesJSON: jsonOr(req.ParentTypesJSON, "[]"),
		Grantable:       req.Grantable,
		Auditable:       req.Auditable,
		SpiceDBType:     strings.TrimSpace(req.SpiceDBType),
		RelationsJSON:   jsonOr(req.RelationsJSON, "[]"),
		PermissionsJSON: jsonOr(req.PermissionsJSON, "[]"),
		MetadataSchema:  jsonOr(req.MetadataSchema, "{}"),
		Status:          data.StatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if rt.OwnerService == "" {
		rt.OwnerService = "aisphere-iam"
	}
	if err := s.repo.UpsertResourceType(ctx, rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func (s *Service) UpsertResource(ctx context.Context, req UpsertResourceRequest) (*data.ResourceModel, authz.WriteResult, error) {
	if s.repo == nil {
		return nil, authz.WriteResult{}, errors.New("resource service repository is nil")
	}
	req.Ref.Type = strings.TrimSpace(req.Ref.Type)
	req.Ref.ID = strings.TrimSpace(req.Ref.ID)
	if req.Ref.Type == "" {
		return nil, authz.WriteResult{}, errors.New("resource type is required")
	}
	if req.Ref.ID == "" {
		req.Ref.ID = idgen.New(req.Ref.Type)
	}
	rt, err := s.repo.GetResourceType(ctx, req.Ref.Type)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if req.Parent.Type != "" {
		if err := requireListed(rt.ParentTypesJSON, req.Parent.Type, "parent type is not allowed for resource type"); err != nil {
			return nil, authz.WriteResult{}, err
		}
		if err := s.requireResourceExists(ctx, req.Parent, req.OrgID); err != nil {
			return nil, authz.WriteResult{}, err
		}
	}
	now := s.now()
	model := &data.ResourceModel{
		ID:              req.Ref.ID,
		Type:            req.Ref.Type,
		OrgID:           strings.TrimSpace(req.OrgID),
		ProjectID:       strings.TrimSpace(req.ProjectID),
		ParentType:      strings.TrimSpace(req.Parent.Type),
		ParentID:        strings.TrimSpace(req.Parent.ID),
		OwnerService:    nonEmpty(req.OwnerService, rt.OwnerService),
		OwnerResourceID: nonEmpty(req.OwnerResourceID, req.Ref.ID),
		Slug:            strings.TrimSpace(req.Slug),
		DisplayName:     strings.TrimSpace(req.DisplayName),
		Path:            strings.TrimSpace(req.Path),
		Status:          nonEmpty(req.Status, data.StatusActive),
		Visibility:      nonEmpty(req.Visibility, "private"),
		LabelsJSON:      jsonOr(req.LabelsJSON, "{}"),
		AnnotationsJSON: jsonOr(req.AnnotationsJSON, "{}"),
		MetadataJSON:    jsonOr(req.MetadataJSON, "{}"),
		CreatedBy:       subjectString(req.CreatedBy),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if model.OrgID == "" {
		return nil, authz.WriteResult{}, errors.New("org_id is required")
	}
	rels := make([]authz.Relationship, 0, 2)
	if model.ParentType != "" && model.ParentID != "" {
		parentType, err := s.spiceType(ctx, model.ParentType)
		if err != nil {
			return model, authz.WriteResult{}, err
		}
		rels = append(rels, authz.Relationship{
			Resource: graph.Object(rt.SpiceDBType, model.ID),
			Relation: RelationParent,
			Subject:  graph.Subject(parentType, model.ParentID, ""),
		})
	}
	if !subjectZero(req.Owner) {
		rels = append(rels, authz.Relationship{
			Resource: graph.Object(rt.SpiceDBType, model.ID),
			Relation: "owner",
			Subject:  toAuthzSubject(req.Owner),
		})
	}
	event, err := s.projection.NewWriteEvent("resource", model.Type+":"+model.ID, rels...)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if err := s.repo.UpsertResource(ctx, model, event); err != nil {
		return nil, authz.WriteResult{}, err
	}
	wr, err := s.projection.Dispatch(ctx, event)
	return model, wr, err
}

func (s *Service) BindResource(ctx context.Context, req BindResourceRequest) (*data.ResourceBindingModel, authz.WriteResult, error) {
	if s.repo == nil {
		return nil, authz.WriteResult{}, errors.New("resource service repository is nil")
	}
	req.Source.Type, req.Source.ID = strings.TrimSpace(req.Source.Type), strings.TrimSpace(req.Source.ID)
	req.Target.Type, req.Target.ID = strings.TrimSpace(req.Target.Type), strings.TrimSpace(req.Target.ID)
	req.Relation = strings.TrimSpace(req.Relation)
	if req.Source.Type == "" || req.Source.ID == "" || req.Target.Type == "" || req.Target.ID == "" || req.Relation == "" {
		return nil, authz.WriteResult{}, errors.New("source, target and relation are required")
	}
	sourceType, err := s.repo.GetResourceType(ctx, req.Source.Type)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if err := requireListed(sourceType.RelationsJSON, req.Relation, "relation is not allowed for source resource type"); err != nil {
		return nil, authz.WriteResult{}, err
	}
	if err := s.requireResourceExists(ctx, req.Source, req.OrgID); err != nil {
		return nil, authz.WriteResult{}, err
	}
	if err := s.requireResourceExists(ctx, req.Target, req.OrgID); err != nil {
		return nil, authz.WriteResult{}, err
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = idgen.New("binding")
	}
	now := s.now()
	binding := &data.ResourceBindingModel{
		ID:         id,
		OrgID:      req.OrgID,
		SourceType: req.Source.Type,
		SourceID:   req.Source.ID,
		Relation:   req.Relation,
		TargetType: req.Target.Type,
		TargetID:   req.Target.ID,
		Status:     data.StatusActive,
		CreatedBy:  subjectString(req.CreatedBy),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	rel, err := s.bindingRelationship(ctx, req)
	if err != nil {
		return binding, authz.WriteResult{}, err
	}
	event, err := s.projection.NewWriteEvent("resource_binding", binding.ID, rel)
	if err != nil {
		return nil, authz.WriteResult{}, err
	}
	if err := s.repo.BindResource(ctx, binding, event); err != nil {
		return nil, authz.WriteResult{}, err
	}
	wr, err := s.projection.Dispatch(ctx, event)
	return binding, wr, err
}

// DeleteResource removes the resource record and purges every SpiceDB
// relationship where the resource appears as a subject or object, so moving
// or deleting a resource no longer leaves dangling authorization tuples.
// The deleted relationships are captured as compensation data so a DTM saga
// rollback can restore them.
func (s *Service) DeleteResource(ctx context.Context, ref ResourceRef, orgID string) error {
	if s.repo == nil {
		return errors.New("resource service repository is nil")
	}
	ref.Type, ref.ID = strings.TrimSpace(ref.Type), strings.TrimSpace(ref.ID)
	if ref.Type == "" || ref.ID == "" {
		return errors.New("resource type and id are required")
	}
	spiceType, err := s.spiceType(ctx, ref.Type)
	if err != nil {
		return err
	}
	filters := []authz.RelationshipFilter{
		{ResourceType: spiceType, ResourceID: ref.ID},
		{SubjectType: spiceType, SubjectID: ref.ID},
	}
	rels, _ := s.captureRelationships(ctx, filters)
	event, err := s.projection.NewBatchDeleteEvent("resource", ref.Type+":"+ref.ID, filters, rels...)
	if err != nil {
		return err
	}
	if err := s.repo.CreateOutboxEvents(ctx, event); err != nil {
		return err
	}
	if _, err := s.projection.Dispatch(ctx, event); err != nil {
		return err
	}
	return s.repo.DeleteResource(ctx, ref.Type, ref.ID, orgID)
}

// ArchiveResource marks the resource archived and purges its authorization
// relationships so archived resources immediately lose all grants.  There is
// no un-archive path today; compensation data is retained for a future restore.
func (s *Service) ArchiveResource(ctx context.Context, ref ResourceRef, orgID string) error {
	if s.repo == nil {
		return errors.New("resource service repository is nil")
	}
	ref.Type, ref.ID = strings.TrimSpace(ref.Type), strings.TrimSpace(ref.ID)
	if ref.Type == "" || ref.ID == "" {
		return errors.New("resource type and id are required")
	}
	spiceType, err := s.spiceType(ctx, ref.Type)
	if err != nil {
		return err
	}
	filters := []authz.RelationshipFilter{
		{ResourceType: spiceType, ResourceID: ref.ID},
		{SubjectType: spiceType, SubjectID: ref.ID},
	}
	rels, _ := s.captureRelationships(ctx, filters)
	event, err := s.projection.NewBatchDeleteEvent("resource", ref.Type+":"+ref.ID, filters, rels...)
	if err != nil {
		return err
	}
	if err := s.repo.CreateOutboxEvents(ctx, event); err != nil {
		return err
	}
	if _, err := s.projection.Dispatch(ctx, event); err != nil {
		return err
	}
	return s.repo.ArchiveResource(ctx, ref.Type, ref.ID, orgID)
}

// MoveResource reparents the resource and projects a parent-relationship
// replace (delete the old #parent edge, write the new one) so SpiceDB topology
// stays in sync with the DB record.
func (s *Service) MoveResource(ctx context.Context, ref, newParent ResourceRef, orgID string) (*data.ResourceModel, error) {
	if s.repo == nil {
		return nil, errors.New("resource service repository is nil")
	}
	ref.Type, ref.ID = strings.TrimSpace(ref.Type), strings.TrimSpace(ref.ID)
	if ref.Type == "" || ref.ID == "" {
		return nil, errors.New("resource type and id are required")
	}
	existing, err := s.repo.GetResource(ctx, ref.Type, ref.ID, orgID)
	if err != nil {
		return nil, err
	}
	spiceType, err := s.spiceType(ctx, ref.Type)
	if err != nil {
		return nil, err
	}
	var previous, desired []authz.Relationship
	if strings.TrimSpace(existing.ParentType) != "" && strings.TrimSpace(existing.ParentID) != "" {
		oldParentType, err := s.spiceType(ctx, existing.ParentType)
		if err != nil {
			return nil, err
		}
		previous = append(previous, authz.Relationship{
			Resource: graph.Object(spiceType, existing.ID),
			Relation: RelationParent,
			Subject:  graph.Subject(oldParentType, existing.ParentID, ""),
		})
	}
	existing.ParentType = strings.TrimSpace(newParent.Type)
	existing.ParentID = strings.TrimSpace(newParent.ID)
	if existing.ParentType != "" && existing.ParentID != "" {
		newParentType, err := s.spiceType(ctx, existing.ParentType)
		if err != nil {
			return nil, err
		}
		desired = append(desired, authz.Relationship{
			Resource: graph.Object(spiceType, existing.ID),
			Relation: RelationParent,
			Subject:  graph.Subject(newParentType, existing.ParentID, ""),
		})
	}
	if err := s.repo.UpsertResource(ctx, existing); err != nil {
		return nil, err
	}
	if len(previous) > 0 || len(desired) > 0 {
		event, err := s.projection.NewReplaceEvent("resource", ref.Type+":"+ref.ID, previous, desired)
		if err != nil {
			return existing, err
		}
		if err := s.repo.CreateOutboxEvents(ctx, event); err != nil {
			return existing, err
		}
		if _, err := s.projection.Dispatch(ctx, event); err != nil {
			return existing, err
		}
	}
	return existing, nil
}

// UnbindResource removes a resource binding and projects deletion of the
// SpiceDB relationship that the binding established.
func (s *Service) UnbindResource(ctx context.Context, bindingID, orgID string) error {
	if s.repo == nil {
		return errors.New("resource service repository is nil")
	}
	bindingID = strings.TrimSpace(bindingID)
	if bindingID == "" {
		return errors.New("binding id is required")
	}
	bindings, err := s.repo.ListResourceBindings(ctx, data.ListOptions{OrgID: orgID, Status: data.StatusActive})
	if err != nil {
		return err
	}
	var match *data.ResourceBindingModel
	for i := range bindings {
		if bindings[i].ID == bindingID {
			match = &bindings[i]
			break
		}
	}
	if match == nil {
		// Binding may already be gone; still clear the DB row idempotently.
		return s.repo.UnbindResource(ctx, bindingID, orgID)
	}
	srcType, err := s.spiceType(ctx, match.SourceType)
	if err != nil {
		return err
	}
	tgtType, err := s.spiceType(ctx, match.TargetType)
	if err != nil {
		return err
	}
	relation := match.Relation
	subject := graph.Subject(tgtType, match.TargetID, "")
	rel := authz.Relationship{
		Resource: graph.Object(srcType, match.SourceID),
		Relation: relation,
		Subject:  subject,
	}
	filter := authz.RelationshipFilter{
		ResourceType: srcType, ResourceID: match.SourceID, Relation: relation,
		SubjectType: subject.Type, SubjectID: subject.ID, SubjectRel: subject.Relation,
	}
	event, err := s.projection.NewDeleteEvent("resource_binding", bindingID, filter, rel)
	if err != nil {
		return err
	}
	if err := s.repo.CreateOutboxEvents(ctx, event); err != nil {
		return err
	}
	if _, err := s.projection.Dispatch(ctx, event); err != nil {
		return err
	}
	return s.repo.UnbindResource(ctx, bindingID, orgID)
}

// captureRelationships reads current SpiceDB relationships for the given
// filters so they can be attached as compensation data to delete events.  It
// is best-effort: a missing reader or read error yields nil compensation
// rather than blocking the lifecycle operation.
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

func (s *Service) BindExternalResource(ctx context.Context, req BindExternalResourceRequest) (*data.ExternalResourceBindingModel, error) {
	if s.repo == nil {
		return nil, errors.New("resource service repository is nil")
	}
	if strings.TrimSpace(req.Resource.Type) == "" || strings.TrimSpace(req.Resource.ID) == "" || strings.TrimSpace(req.Provider) == "" || strings.TrimSpace(req.ExternalType) == "" || strings.TrimSpace(req.ExternalID) == "" {
		return nil, errors.New("resource, provider, external_type and external_id are required")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = idgen.New("extbind")
	}
	now := s.now()
	b := &data.ExternalResourceBindingModel{
		ID:           id,
		OrgID:        req.OrgID,
		ResourceType: strings.TrimSpace(req.Resource.Type),
		ResourceID:   strings.TrimSpace(req.Resource.ID),
		Provider:     strings.TrimSpace(req.Provider),
		ExternalType: strings.TrimSpace(req.ExternalType),
		ExternalID:   strings.TrimSpace(req.ExternalID),
		ExternalPath: strings.TrimSpace(req.ExternalPath),
		ExternalURL:  strings.TrimSpace(req.ExternalURL),
		SyncMode:     nonEmpty(req.SyncMode, "owner"),
		SyncStatus:   data.StatusPending,
		MetadataJSON: jsonOr(req.MetadataJSON, "{}"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.BindExternalResource(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Service) spiceType(ctx context.Context, typ string) (string, error) {
	switch strings.TrimSpace(typ) {
	case "organization", "project":
		return strings.TrimSpace(typ), nil
	}
	rt, err := s.repo.GetResourceType(ctx, typ)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(rt.SpiceDBType) == "" {
		return rt.Type, nil
	}
	return rt.SpiceDBType, nil
}

func (s *Service) bindingRelationship(ctx context.Context, req BindResourceRequest) (authz.Relationship, error) {
	srcType, err := s.spiceType(ctx, req.Source.Type)
	if err != nil {
		return authz.Relationship{}, err
	}
	tgtType, err := s.spiceType(ctx, req.Target.Type)
	if err != nil {
		return authz.Relationship{}, err
	}

	return authz.Relationship{
		Resource: graph.Object(srcType, req.Source.ID),
		Relation: req.Relation,
		Subject:  graph.Subject(tgtType, req.Target.ID, ""),
	}, nil
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(fallback)
}
func jsonOr(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
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

func (s *Service) requireResourceExists(ctx context.Context, ref ResourceRef, orgID string) error {
	switch strings.TrimSpace(ref.Type) {
	case "":
		return errors.New("resource type is required")
	case "organization":
		return errors.New("resource type organization is removed; use zone")
	case "zone", "group":
		if strings.TrimSpace(ref.ID) == "" {
			return errors.New("resource id is required")
		}
		return nil
	case "project":
		_, err := s.repo.GetProject(ctx, ref.ID, orgID)
		return err
	default:
		_, err := s.repo.GetResource(ctx, ref.Type, ref.ID, orgID)
		return err
	}
}

func requireListed(raw, value, message string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(jsonOr(raw, "[]")), &items); err != nil {
		return err
	}
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			return nil
		}
	}
	return errors.New(message + ": " + value)
}

func firstProjectionManager(managers []*projection.Manager) *projection.Manager {
	for _, manager := range managers {
		if manager != nil {
			return manager
		}
	}
	return nil
}
