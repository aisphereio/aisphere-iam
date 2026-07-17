package data

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/aisphereio/kernel/authn"
)

func TestGroupMetadataMigrationAccommodatesStableIDs(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/000011_iam_groups_widen_id.sql")
	if err != nil {
		t.Fatalf("read group ID widening migration: %v", err)
	}
	text := strings.ToUpper(string(migration))
	if !strings.Contains(text, "ALTER COLUMN ID TYPE VARCHAR(64)") {
		t.Fatalf("migration does not widen iam_groups.id for grp_ + 32 hex stable IDs:\n%s", text)
	}
}

func TestGroupMetadataIdentityAdminPersistsAndRestoresExternalID(t *testing.T) {
	repo := newFakeGroupMetadataRepository()
	provider := &metadataLosingIdentity{}
	admin := BindGroupMetadata(provider, repo)

	created, err := admin.CreateGroup(context.Background(), authn.CreateGroupRequest{
		Group: authn.Group{
			ID:          "grp_stable",
			ExternalID:  "engineering",
			OrgID:       "aisphere",
			Name:        "grp_stable",
			DisplayName: "Engineering",
			Type:        "Physical",
		},
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if created.ExternalID != "engineering" {
		t.Fatalf("created.ExternalID = %q, want engineering", created.ExternalID)
	}
	stored := repo.groups["aisphere/grp_stable"]
	if stored == nil {
		t.Fatal("group metadata was not persisted")
	}
	if stored.Name != "engineering" || stored.CasdoorName != "grp_stable" {
		t.Fatalf("stored metadata = %#v", stored)
	}

	groups, err := admin.ListGroups(context.Background(), authn.GroupFilter{OrgID: "aisphere"})
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].ExternalID != "engineering" {
		t.Fatalf("groups = %#v, want external ID restored from IAM metadata", groups)
	}
}

func TestGroupMetadataIdentityAdminPreservesExternalIDWhenUpdateOmitsName(t *testing.T) {
	repo := newFakeGroupMetadataRepository()
	repo.groups["aisphere/grp_stable"] = &GroupModel{
		ID:          "grp_stable",
		OrgID:       "aisphere",
		Name:        "engineering",
		DisplayName: "Engineering",
		CasdoorName: "grp_stable",
		Type:        "Physical",
		Status:      StatusActive,
	}
	provider := &metadataLosingIdentity{group: authn.Group{
		ID:          "grp_stable",
		OrgID:       "aisphere",
		Name:        "grp_stable",
		DisplayName: "Engineering",
		Type:        "Physical",
	}}
	admin := BindGroupMetadata(provider, repo)

	updated, err := admin.UpdateGroup(context.Background(), authn.UpdateGroupRequest{
		Group: authn.Group{
			ID:          "grp_stable",
			OrgID:       "aisphere",
			Name:        "grp_stable",
			DisplayName: "Engineering Platform",
			Type:        "Physical",
		},
	})
	if err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	if updated.ExternalID != "engineering" {
		t.Fatalf("updated.ExternalID = %q, want existing engineering alias", updated.ExternalID)
	}
	if got := repo.groups["aisphere/grp_stable"].Name; got != "engineering" {
		t.Fatalf("stored name = %q, want engineering", got)
	}
}

type metadataLosingIdentity struct {
	authn.IdentityAdmin
	group authn.Group
}

func (m *metadataLosingIdentity) CreateGroup(_ context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	m.group = req.Group
	m.group.ExternalID = ""
	return m.group, nil
}

func (m *metadataLosingIdentity) ListGroups(context.Context, authn.GroupFilter) ([]authn.Group, error) {
	return []authn.Group{m.group}, nil
}

func (m *metadataLosingIdentity) UpdateGroup(_ context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	m.group = req.Group
	m.group.ExternalID = ""
	return m.group, nil
}

type fakeGroupMetadataRepository struct {
	groups map[string]*GroupModel
}

func newFakeGroupMetadataRepository() *fakeGroupMetadataRepository {
	return &fakeGroupMetadataRepository{groups: map[string]*GroupModel{}}
}

func (r *fakeGroupMetadataRepository) SaveGroup(_ context.Context, group *GroupModel) error {
	copy := *group
	r.groups[group.OrgID+"/"+group.ID] = &copy
	return nil
}

func (r *fakeGroupMetadataRepository) ListGroupMetadata(_ context.Context, orgID string) ([]GroupModel, error) {
	out := make([]GroupModel, 0, len(r.groups))
	for _, group := range r.groups {
		if group.OrgID == orgID && group.Status == StatusActive {
			out = append(out, *group)
		}
	}
	return out, nil
}

func (r *fakeGroupMetadataRepository) ArchiveGroup(_ context.Context, orgID, groupID string) error {
	if group := r.groups[orgID+"/"+groupID]; group != nil {
		group.Status = StatusArchived
	}
	return nil
}
