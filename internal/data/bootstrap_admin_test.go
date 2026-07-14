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
		DefaultRole: "zone_owner",
		Roles: map[string]permissionmanifest.BootstrapRole{
			"zone_owner": {
				ZoneRelations:     []string{"owner", "permission_admin"},
				ControlPlaneAdmin: true,
			},
		},
		AdminResources: []permissionmanifest.AdminResource{{Type: "iam_authz", ID: "global"}},
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
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "owner", Subject: authz.SubjectRef{Type: "user", ID: "u1"}},
		{Resource: authz.ObjectRef{Type: "zone", ID: "aisphere"}, Relation: "permission_admin", Subject: authz.SubjectRef{Type: "user", ID: "u1"}},
		{Resource: authz.ObjectRef{Type: "iam_authz", ID: "global"}, Relation: "admin", Subject: authz.SubjectRef{Type: "user", ID: "u1"}},
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

func TestBootstrapControlPlaneAdminsRejectsUnknownManifestRole(t *testing.T) {
	policy := permissionmanifest.BootstrapPolicy{
		DefaultRole: "zone_owner",
		Roles: map[string]permissionmanifest.BootstrapRole{
			"zone_owner": {ZoneRelations: []string{"owner"}},
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
	if manifest == nil || len(manifest.Bootstrap.AdminResources) != 9 {
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
