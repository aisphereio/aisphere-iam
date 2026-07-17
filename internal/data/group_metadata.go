package data

import (
	"context"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/dbx"
)

// GroupMetadataRepository owns the IAM-side mapping between stable group IDs
// and user-facing machine names. Casdoor stores the stable ID in Group.Name,
// so this mapping must live in IAM rather than in the provider adapter.
type GroupMetadataRepository interface {
	SaveGroup(ctx context.Context, group *GroupModel) error
	ListGroupMetadata(ctx context.Context, orgID string) ([]GroupModel, error)
	ArchiveGroup(ctx context.Context, orgID, groupID string) error
}

type DBGroupMetadataRepository struct {
	db dbx.DB
}

func NewGroupMetadataRepository(db dbx.DB) *DBGroupMetadataRepository {
	return &DBGroupMetadataRepository{db: db}
}

func (r *DBGroupMetadataRepository) SaveGroup(ctx context.Context, group *GroupModel) error {
	return r.db.SafeUpsert(ctx, group, []string{
		"parent_id", "name", "display_name", "casdoor_name", "type",
		"status", "updated_at", "deleted_at",
	})
}

func (r *DBGroupMetadataRepository) ListGroupMetadata(ctx context.Context, orgID string) ([]GroupModel, error) {
	var out []GroupModel
	return out, r.db.FindMany(ctx, &out, "org_id = ? AND status = ?", orgID, StatusActive)
}

func (r *DBGroupMetadataRepository) ArchiveGroup(ctx context.Context, orgID, groupID string) error {
	now := time.Now().UTC()
	return r.db.Update(ctx, &GroupModel{}, "org_id = ? AND id = ?", []any{orgID, groupID}, map[string]any{
		"status":     StatusArchived,
		"deleted_at": now,
		"updated_at": now,
	})
}

// BindGroupMetadata decorates a provider-neutral identity directory with the
// IAM-owned group metadata mapping. The Kernel provider remains responsible
// only for fields that Casdoor actually persists.
func BindGroupMetadata(next authn.IdentityAdmin, repo GroupMetadataRepository) authn.IdentityAdmin {
	if next == nil || repo == nil {
		return next
	}
	return groupMetadataIdentityAdmin{IdentityAdmin: next, repo: repo}
}

type groupMetadataIdentityAdmin struct {
	authn.IdentityAdmin
	repo GroupMetadataRepository
}

func (a groupMetadataIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	group, err := a.IdentityAdmin.CreateGroup(ctx, req)
	if err != nil {
		return authn.Group{}, err
	}
	model := groupMetadataModel(group, req.Group)
	if err := a.repo.SaveGroup(ctx, model); err != nil {
		return authn.Group{}, authn.ErrIdentityBackendFailed("persist IAM group metadata failed", err)
	}
	return decorateGroupMetadata(group, *model), nil
}

func (a groupMetadataIdentityAdmin) GetGroup(ctx context.Context, orgID, groupID string) (authn.Group, error) {
	group, err := a.IdentityAdmin.GetGroup(ctx, orgID, groupID)
	if err != nil {
		return authn.Group{}, err
	}
	metadata, err := a.metadataByProviderID(ctx, orgID)
	if err != nil {
		return authn.Group{}, err
	}
	if model, ok := lookupGroupMetadata(metadata, group); ok {
		group = decorateGroupMetadata(group, model)
	}
	return group, nil
}

func (a groupMetadataIdentityAdmin) ListGroups(ctx context.Context, filter authn.GroupFilter) ([]authn.Group, error) {
	groups, err := a.IdentityAdmin.ListGroups(ctx, filter)
	if err != nil {
		return nil, err
	}
	metadata, err := a.metadataByProviderID(ctx, filter.OrgID)
	if err != nil {
		return nil, err
	}
	for i, group := range groups {
		if model, ok := lookupGroupMetadata(metadata, group); ok {
			groups[i] = decorateGroupMetadata(group, model)
		}
	}
	return groups, nil
}

func (a groupMetadataIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	metadata, err := a.metadataByProviderID(ctx, req.Group.OrgID)
	if err != nil {
		return authn.Group{}, err
	}
	group, err := a.IdentityAdmin.UpdateGroup(ctx, req)
	if err != nil {
		return authn.Group{}, err
	}
	model := groupMetadataModel(group, req.Group)
	if strings.TrimSpace(req.Group.ExternalID) == "" {
		if existing, ok := lookupGroupMetadata(metadata, group); ok {
			model.Name = existing.Name
			model.CreatedAt = existing.CreatedAt
		}
	}
	if err := a.repo.SaveGroup(ctx, model); err != nil {
		return authn.Group{}, authn.ErrIdentityBackendFailed("persist IAM group metadata failed", err)
	}
	return decorateGroupMetadata(group, *model), nil
}

func (a groupMetadataIdentityAdmin) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	if err := a.IdentityAdmin.DeleteGroup(ctx, req); err != nil {
		return err
	}
	if err := a.repo.ArchiveGroup(ctx, req.OrgID, req.GroupID); err != nil {
		return authn.ErrIdentityBackendFailed("archive IAM group metadata failed", err)
	}
	return nil
}

func (a groupMetadataIdentityAdmin) metadataByProviderID(ctx context.Context, orgID string) (map[string]GroupModel, error) {
	models, err := a.repo.ListGroupMetadata(ctx, orgID)
	if err != nil {
		return nil, authn.ErrIdentityBackendFailed("list IAM group metadata failed", err)
	}
	out := make(map[string]GroupModel, len(models)*2)
	for _, model := range models {
		for _, key := range []string{model.ID, model.CasdoorName} {
			if key = strings.TrimSpace(key); key != "" {
				out[key] = model
			}
		}
	}
	return out, nil
}

func lookupGroupMetadata(metadata map[string]GroupModel, group authn.Group) (GroupModel, bool) {
	for _, key := range []string{group.ID, group.Name} {
		if model, ok := metadata[strings.TrimSpace(key)]; ok {
			return model, true
		}
	}
	return GroupModel{}, false
}

func groupMetadataModel(group, requested authn.Group) *GroupModel {
	now := time.Now().UTC()
	id := firstGroupValue(group.ID, requested.ID, group.Name, requested.Name)
	providerName := firstGroupValue(group.Name, requested.Name, id)
	externalID := firstGroupValue(requested.ExternalID, group.ExternalID)
	if externalID == "" || externalID == id {
		externalID = id
	}
	return &GroupModel{
		ID:          id,
		OrgID:       firstGroupValue(group.OrgID, requested.OrgID),
		ParentID:    requested.ParentID,
		Name:        externalID,
		DisplayName: firstGroupValue(requested.DisplayName, group.DisplayName, externalID),
		CasdoorName: providerName,
		Type:        firstGroupValue(requested.Type, group.Type, "Physical"),
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func decorateGroupMetadata(group authn.Group, model GroupModel) authn.Group {
	group.ID = firstGroupValue(group.ID, model.ID)
	group.ExternalID = firstGroupValue(model.Name, group.ExternalID)
	group.OrgID = firstGroupValue(model.OrgID, group.OrgID)
	group.ParentID = model.ParentID
	group.DisplayName = firstGroupValue(model.DisplayName, group.DisplayName)
	group.Type = firstGroupValue(model.Type, group.Type)
	return group
}

func firstGroupValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

var _ authn.IdentityAdmin = groupMetadataIdentityAdmin{}
var _ GroupMetadataRepository = (*DBGroupMetadataRepository)(nil)
