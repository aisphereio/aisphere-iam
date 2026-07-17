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

func TestCommittedSkillModelIsCanonicalGitAuthorizationResource(t *testing.T) {
	manifest, schema := loadCommittedManifestAndSchema(t)
	for _, removed := range []string{"git_namespace", "git_repository"} {
		if _, ok := schema.Definitions[removed]; ok {
			t.Fatalf("removed SpiceDB definition %s is still present", removed)
		}
		for _, resourceType := range manifest.ResourceTypes {
			if resourceType.Type == removed || resourceType.SpiceDBType == removed {
				t.Fatalf("removed resource type %s is still registered", removed)
			}
		}
	}

	skill, ok := schema.Definitions["skill"]
	if !ok {
		t.Fatal("skill definition is missing")
	}
	wantRelations := map[string]string{
		"owner":     "user|service|service_account|group#member",
		"editor":    "user|service|service_account|group#member",
		"reviewer":  "user|service|service_account|group#member",
		"publisher": "user|service|service_account|group#member",
		"viewer":    "user|user:*|service|service:*|service_account|service_account:*|group#member",
	}
	for relation, want := range wantRelations {
		if got := skill.Relations[relation]; got != want {
			t.Fatalf("skill relation %s = %q, want %q", relation, got, want)
		}
	}
	if got, want := skill.Permissions["publish"], "manage+publisher+custom_binding->publish"; got != want {
		t.Fatalf("skill publish permission = %q, want %q", got, want)
	}
	if strings.Contains(skill.Permissions["publish"], "reviewer") {
		t.Fatal("reviewer must not imply publish")
	}

	var skillResource *ResourceType
	for i := range manifest.ResourceTypes {
		if manifest.ResourceTypes[i].Type == "skill" {
			skillResource = &manifest.ResourceTypes[i]
			break
		}
	}
	if skillResource == nil {
		t.Fatal("skill resource type is missing")
	}
	if !slices.Contains(skillResource.Relations, "publisher") {
		t.Fatal("skill resource type does not expose publisher relation")
	}
}

func TestGrantableResourcesExposeCustomRoleBindings(t *testing.T) {
	manifest, schema := loadCommittedManifestAndSchema(t)
	for _, resourceType := range manifest.ResourceTypes {
		if !resourceType.Grantable {
			continue
		}
		definition := schema.Definitions[resourceType.SpiceDBType]
		if _, ok := definition.Relations["custom_binding"]; !ok {
			t.Fatalf("grantable resource %s has no custom_binding relation", resourceType.Type)
		}
		for _, permission := range resourceType.Permissions {
			if !strings.Contains(definition.Permissions[permission], "custom_binding->"+permission) {
				t.Fatalf("permission %s#%s does not include its custom binding", resourceType.SpiceDBType, permission)
			}
		}
	}
	if _, ok := schema.Definitions["custom_role"]; !ok {
		t.Fatal("custom_role definition is missing")
	}
	if _, ok := schema.Definitions["role_binding"]; !ok {
		t.Fatal("role_binding definition is missing")
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
