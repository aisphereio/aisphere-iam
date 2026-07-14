package permissionmanifest

import (
	"path/filepath"
	"slices"
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
