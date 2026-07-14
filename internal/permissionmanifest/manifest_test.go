package permissionmanifest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadResolvesBootstrapRoleAliases(t *testing.T) {
	manifest, err := Load(filepath.Join("..", "..", "configs", "resource", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	role, canonical, ok := manifest.ResolveBootstrapRole("owner")
	if !ok || canonical != "zone_owner" || !role.ControlPlaneAdmin {
		t.Fatalf("resolved = %#v, %q, %v", role, canonical, ok)
	}
	want := []string{"owner", "admin", "user_manager", "group_manager", "permission_admin"}
	if !slices.Equal(want, role.ZoneRelations) {
		t.Fatalf("zone relations = %v, want %v", role.ZoneRelations, want)
	}
	if len(manifest.Bootstrap.AdminResources) != 9 {
		t.Fatalf("admin resources = %d, want 9", len(manifest.Bootstrap.AdminResources))
	}
}

func TestResolveBootstrapRoleUsesConfiguredDefault(t *testing.T) {
	manifest := Manifest{Bootstrap: BootstrapPolicy{
		DefaultRole: "zone_owner",
		Roles: map[string]BootstrapRole{
			"zone_owner": {ZoneRelations: []string{"owner"}},
		},
	}}

	_, canonical, ok := manifest.ResolveBootstrapRole("")
	if !ok || canonical != "zone_owner" {
		t.Fatalf("canonical = %q, ok = %v", canonical, ok)
	}
}

func TestCommittedManifestMatchesSpiceDBSchema(t *testing.T) {
	manifest, schema := loadCommittedManifestAndSchema(t)
	if err := Validate(manifest, schema); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsPermissionCatalogDrift(t *testing.T) {
	manifest, schema := loadCommittedManifestAndSchema(t)
	for i := range manifest.ResourceTypes {
		if manifest.ResourceTypes[i].Type == "zone" {
			manifest.ResourceTypes[i].Permissions = manifest.ResourceTypes[i].Permissions[1:]
		}
	}

	err := Validate(manifest, schema)
	if err == nil || !strings.Contains(err.Error(), "resource type zone permissions") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsDuplicateBootstrapAlias(t *testing.T) {
	manifest, schema := loadCommittedManifestAndSchema(t)
	role := manifest.Bootstrap.Roles["zone_admin"]
	role.Aliases = append(role.Aliases, "owner")
	manifest.Bootstrap.Roles["zone_admin"] = role

	err := Validate(manifest, schema)
	if err == nil || !strings.Contains(err.Error(), "duplicate bootstrap role name or alias owner") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsUnknownBootstrapResource(t *testing.T) {
	manifest, schema := loadCommittedManifestAndSchema(t)
	manifest.Bootstrap.AdminResources = append(manifest.Bootstrap.AdminResources, AdminResource{Type: "missing", ID: "global"})

	err := Validate(manifest, schema)
	if err == nil || !strings.Contains(err.Error(), "bootstrap admin resource type missing") {
		t.Fatalf("error = %v", err)
	}
}

func loadCommittedManifestAndSchema(t *testing.T) (*Manifest, Schema) {
	t.Helper()
	root := filepath.Join("..", "..")
	manifest, err := Load(filepath.Join(root, "configs", "resource", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "configs", "spicedb", "aisphere.schema.zed"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := ParseSchema(string(body))
	if err != nil {
		t.Fatal(err)
	}
	return manifest, schema
}
