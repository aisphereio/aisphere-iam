package data

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/logx"

	"github.com/aisphereio/aisphere-iam/internal/conf"
	"github.com/aisphereio/aisphere-iam/internal/permissionmanifest"
)

func TestBootstrapControlPlaneAdminsUsesManifestPolicy(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	policy := permissionmanifest.BootstrapPolicy{
		DefaultRole: "platform_owner",
		PlatformID:  "global",
		Roles: map[string]permissionmanifest.BootstrapRole{
			"platform_owner": {Scope: "platform", Relation: "owner"},
			"zone_owner":     {Scope: "zone", Relation: "owner"},
		},
		PlatformResources: []permissionmanifest.AdminResource{{Type: "iam_authz", ID: "global"}},
	}
	cfg := conf.ControlPlaneBootstrapAdminsConfig{
		Enabled: true,
		Subjects: []conf.ControlPlaneAdminSubject{{
			Type: "user", ID: "u1", ZoneID: "aisphere", Role: "zone_owner",
		}},
	}

	if err := bootstrapControlPlaneAdmins(ctx, cfg, policy, store, nil, logx.Noop()); err != nil {
		t.Fatal(err)
	}
	relationships, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatal(err)
	}
	want := []authz.Relationship{
		{Resource: authz.ObjectRef{Type: "iam_authz", ID: "global"}, Relation: "platform", Subject: authz.SubjectRef{Type: "platform", ID: "global"}},
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "platform", Subject: authz.SubjectRef{Type: "platform", ID: "global"}},
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "owner", Subject: authz.SubjectRef{Type: "user", ID: "u1"}},
	}
	if len(relationships) != len(want) {
		t.Fatalf("relationships = %#v, want %#v", relationships, want)
	}
	for _, expected := range want {
		if !hasBootstrapRelationship(relationships, expected) {
			t.Fatalf("missing relationship %#v in %#v", expected, relationships)
		}
	}
}

func TestBootstrapControlPlaneAdminsWritesStructuralRelationshipsOnce(t *testing.T) {
	store := authz.NewMemoryRelationshipStore()
	policy := permissionmanifest.BootstrapPolicy{
		DefaultRole: "zone_admin",
		PlatformID:  "global",
		Roles: map[string]permissionmanifest.BootstrapRole{
			"zone_admin": {Scope: "zone", Relation: "admin"},
		},
		PlatformResources: []permissionmanifest.AdminResource{{Type: "iam", ID: "grant"}},
	}
	cfg := conf.ControlPlaneBootstrapAdminsConfig{Enabled: true, Subjects: []conf.ControlPlaneAdminSubject{
		{Type: "user", ID: "u1", ZoneID: "org-a", Role: "zone_admin"},
		{Type: "user", ID: "u2", ZoneID: "org-a", Role: "zone_admin"},
	}}

	if err := bootstrapControlPlaneAdmins(context.Background(), cfg, policy, store, nil, logx.Noop()); err != nil {
		t.Fatal(err)
	}
	relationships, err := store.ReadRelationships(context.Background(), authz.RelationshipFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(relationships) != 4 {
		t.Fatalf("relationships = %#v, want two structural and two personnel relationships", relationships)
	}
}

func TestBootstrapControlPlaneAdminsRejectsZoneRoleWithoutZone(t *testing.T) {
	policy := permissionmanifest.BootstrapPolicy{
		PlatformID: "global",
		Roles: map[string]permissionmanifest.BootstrapRole{
			"zone_admin": {Scope: "zone", Relation: "admin"},
		},
	}
	cfg := conf.ControlPlaneBootstrapAdminsConfig{Enabled: true, Subjects: []conf.ControlPlaneAdminSubject{{Type: "user", ID: "u1", Role: "zone_admin"}}}

	err := bootstrapControlPlaneAdmins(context.Background(), cfg, policy, authz.NewMemoryRelationshipStore(), nil, logx.Noop())
	if err == nil || !strings.Contains(err.Error(), "zone_id is required for bootstrap role zone_admin") {
		t.Fatalf("error = %v", err)
	}
}

func TestBootstrapControlPlaneAdminsCleansOnlyLegacyExpansionRelationships(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	old := []authz.Relationship{
		{Resource: authz.ObjectRef{Type: "zone", ID: "org-a"}, Relation: "admin", Subject: authz.SubjectRef{Type: "user", ID: "u1"}},
		{Resource: authz.ObjectRef{Type: "zone", ID: "org-a"}, Relation: "permission_admin", Subject: authz.SubjectRef{Type: "user", ID: "u1"}},
		{Resource: authz.ObjectRef{Type: "iam_authz", ID: "global"}, Relation: "admin", Subject: authz.SubjectRef{Type: "user", ID: "u1"}},
		{Resource: authz.ObjectRef{Type: "skill", ID: "s1"}, Relation: "viewer", Subject: authz.SubjectRef{Type: "user", ID: "u1"}},
	}
	if _, err := store.WriteRelationships(ctx, old...); err != nil {
		t.Fatal(err)
	}
	policy := permissionmanifest.BootstrapPolicy{
		PlatformID: "global",
		Roles: map[string]permissionmanifest.BootstrapRole{
			"zone_owner": {Scope: "zone", Relation: "owner"},
		},
		PlatformResources: []permissionmanifest.AdminResource{{Type: "iam_authz", ID: "global"}},
	}
	cfg := conf.ControlPlaneBootstrapAdminsConfig{
		Enabled: true, CleanupLegacyExpansions: true,
		Subjects: []conf.ControlPlaneAdminSubject{{Type: "user", ID: "u1", ZoneID: "org-a", Role: "zone_owner"}},
	}

	if err := bootstrapControlPlaneAdmins(ctx, cfg, policy, store, nil, logx.Noop()); err != nil {
		t.Fatal(err)
	}
	relationships, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if hasBootstrapRelationship(relationships, old[0]) || hasBootstrapRelationship(relationships, old[1]) || hasBootstrapRelationship(relationships, old[2]) {
		t.Fatalf("legacy relationships remain: %#v", relationships)
	}
	if !hasBootstrapRelationship(relationships, old[3]) {
		t.Fatalf("unrelated grant was removed: %#v", relationships)
	}
	wantOwner := authz.Relationship{Resource: authz.ObjectRef{Type: "zone", ID: "org-a"}, Relation: "owner", Subject: authz.SubjectRef{Type: "user", ID: "u1"}}
	if !hasBootstrapRelationship(relationships, wantOwner) {
		t.Fatalf("new scoped owner missing: %#v", relationships)
	}
}

func TestBootstrapControlPlaneAdminsCleansLegacyExpansionForPlatformAdmin(t *testing.T) {
	ctx := context.Background()
	store := authz.NewMemoryRelationshipStore()
	// Simulate legacy relationships left by the old zone_owner bootstrap:
	// zone-level expansions + iam/iam_authz admin grants.  Also seed a
	// zone#member (identity projection, NOT a bootstrap expansion) and an
	// unrelated project#owner to verify they are preserved.
	legacy := []authz.Relationship{
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "owner", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}},
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "admin", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}},
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "user_manager", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}},
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "group_manager", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}},
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "permission_admin", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}},
		{Resource: authz.ObjectRef{Type: "iam", ID: "grant"}, Relation: "admin", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}},
		{Resource: authz.ObjectRef{Type: "iam_authz", ID: "global"}, Relation: "admin", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}},
		// Non-legacy — must survive cleanup:
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "member", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}},
		{Resource: authz.ObjectRef{Type: "project", ID: "p1"}, Relation: "owner", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}},
	}
	if _, err := store.WriteRelationships(ctx, legacy...); err != nil {
		t.Fatal(err)
	}
	policy := permissionmanifest.BootstrapPolicy{
		PlatformID: "global",
		Roles: map[string]permissionmanifest.BootstrapRole{
			"platform_admin": {Scope: "platform", Relation: "admin"},
		},
		PlatformResources: []permissionmanifest.AdminResource{
			{Type: "iam", ID: "grant"},
			{Type: "iam_authz", ID: "global"},
		},
	}
	cfg := conf.ControlPlaneBootstrapAdminsConfig{
		Enabled: true, CleanupLegacyExpansions: true,
		Subjects: []conf.ControlPlaneAdminSubject{{Type: "user", ID: "admin-uid", ZoneID: "aisphere", Role: "platform_admin"}},
	}

	if err := bootstrapControlPlaneAdmins(ctx, cfg, policy, store, nil, logx.Noop()); err != nil {
		t.Fatal(err)
	}
	relationships, err := store.ReadRelationships(ctx, authz.RelationshipFilter{})
	if err != nil {
		t.Fatal(err)
	}

	// Legacy zone expansions and iam#admin must be gone.
	for _, old := range legacy[:7] {
		if hasBootstrapRelationship(relationships, old) {
			t.Fatalf("legacy relationship should have been cleaned: %#v", old)
		}
	}
	// zone#member and project#owner must survive.
	if !hasBootstrapRelationship(relationships, legacy[7]) {
		t.Fatalf("zone#member should be preserved: %#v", relationships)
	}
	if !hasBootstrapRelationship(relationships, legacy[8]) {
		t.Fatalf("project#owner should be preserved: %#v", relationships)
	}
	// The new platform#admin relationship must be present.
	wantPlatformAdmin := authz.Relationship{Resource: authz.ObjectRef{Type: "platform", ID: "global"}, Relation: "admin", Subject: authz.SubjectRef{Type: "user", ID: "admin-uid"}}
	if !hasBootstrapRelationship(relationships, wantPlatformAdmin) {
		t.Fatalf("platform#admin missing: %#v", relationships)
	}
}

func TestBootstrapControlPlaneAdminsRejectsUnknownManifestRole(t *testing.T) {
	policy := permissionmanifest.BootstrapPolicy{
		PlatformID: "global",
		Roles: map[string]permissionmanifest.BootstrapRole{
			"zone_owner": {Scope: "zone", Relation: "owner"},
		},
	}
	cfg := conf.ControlPlaneBootstrapAdminsConfig{
		Enabled: true,
		Subjects: []conf.ControlPlaneAdminSubject{{
			Type: "user", ID: "u1", ZoneID: "aisphere", Role: "unknown",
		}},
	}

	err := bootstrapControlPlaneAdmins(context.Background(), cfg, policy, authz.NewMemoryRelationshipStore(), nil, logx.Noop())
	if err == nil || !strings.Contains(err.Error(), "unknown bootstrap role unknown") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadPermissionManifestValidatesCommittedFiles(t *testing.T) {
	root := filepath.Join("..", "..")
	cfg := conf.Bootstrap{}
	cfg.ControlPlane.Defaults.Enabled = true
	cfg.ControlPlane.Defaults.Path = filepath.Join(root, "configs", "resource", "defaults.yaml")
	cfg.Security.Authz.InstallDefaultSchema = true
	cfg.Security.Authz.SchemaPath = filepath.Join(root, "configs", "spicedb", "aisphere.schema.zed")

	manifest, err := loadPermissionManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if manifest == nil || len(manifest.Bootstrap.PlatformResources) != 9 {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func hasBootstrapRelationship(items []authz.Relationship, want authz.Relationship) bool {
	for _, item := range items {
		if reflect.DeepEqual(item, want) {
			return true
		}
	}
	return false
}
