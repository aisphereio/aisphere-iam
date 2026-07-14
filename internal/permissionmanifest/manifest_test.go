package permissionmanifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadResolvesBootstrapRoleAliases(t *testing.T) {
	manifest, err := Load(filepath.Join("..", "..", "configs", "resource", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	role, canonical, ok := manifest.ResolveBootstrapRole("owner")
	if !ok || canonical != "zone_owner" {
		t.Fatalf("resolved = %#v, %q, %v", role, canonical, ok)
	}
	if role.Scope != "zone" || role.Relation != "owner" {
		t.Fatalf("role = %#v, want zone owner", role)
	}
	if manifest.Bootstrap.PlatformID != "global" {
		t.Fatalf("platform id = %q, want global", manifest.Bootstrap.PlatformID)
	}
	if len(manifest.Bootstrap.PlatformResources) != 9 {
		t.Fatalf("platform resources = %d, want 9", len(manifest.Bootstrap.PlatformResources))
	}
}

func TestResolveBootstrapRoleUsesConfiguredDefault(t *testing.T) {
	manifest := Manifest{Bootstrap: BootstrapPolicy{
		DefaultRole: "platform_owner",
		Roles: map[string]BootstrapRole{
			"platform_owner": {Scope: "platform", Relation: "owner"},
		},
	}}

	_, canonical, ok := manifest.ResolveBootstrapRole("")
	if !ok || canonical != "platform_owner" {
		t.Fatalf("canonical = %q, ok = %v", canonical, ok)
	}
}

func TestLoadRequiresExplicitPlatformAdministratorRole(t *testing.T) {
	manifest, err := Load(filepath.Join("..", "..", "configs", "resource", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	role, canonical, ok := manifest.ResolveBootstrapRole("platform_admin")
	if !ok || canonical != "platform_admin" || role.Scope != "platform" || role.Relation != "admin" {
		t.Fatalf("resolved = %#v, %q, %v", role, canonical, ok)
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
	manifest.Bootstrap.PlatformResources = append(manifest.Bootstrap.PlatformResources, AdminResource{Type: "missing", ID: "global"})

	err := Validate(manifest, schema)
	if err == nil || !strings.Contains(err.Error(), "bootstrap platform resource type missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsUnknownBootstrapScope(t *testing.T) {
	manifest, schema := loadCommittedManifestAndSchema(t)
	role := manifest.Bootstrap.Roles["zone_admin"]
	role.Scope = "group"
	manifest.Bootstrap.Roles["zone_admin"] = role

	err := Validate(manifest, schema)
	if err == nil || !strings.Contains(err.Error(), "bootstrap role zone_admin has unsupported scope group") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsPlatformResourceWithoutPlatformRelation(t *testing.T) {
	manifest, schema := loadCommittedManifestAndSchema(t)
	definition := schema.Definitions["iam"]
	delete(definition.Relations, "platform")
	schema.Definitions["iam"] = definition

	err := Validate(manifest, schema)
	if err == nil || !strings.Contains(err.Error(), "bootstrap platform resource iam requires relation platform") {
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
