package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthorizationModelHasSingleOrganizationRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "..")
	schema := mustReadContractFile(t, filepath.Join(root, "configs", "spicedb", "aisphere.schema.zed"))
	defaults := mustReadContractFile(t, filepath.Join(root, "configs", "resource", "defaults.yaml"))

	forbidden := []string{
		"definition organization",
		"relation parent: organization",
	}
	for _, token := range forbidden {
		if strings.Contains(schema, token) {
			t.Fatalf("SpiceDB schema contains removed platform-organization model %q", token)
		}
	}

	requiredSchema := []string{
		"definition zone",
		"definition group",
		"definition project",
		"relation zone: zone",
	}
	for _, token := range requiredSchema {
		if !strings.Contains(schema, token) {
			t.Fatalf("SpiceDB schema is missing required model contract %q", token)
		}
	}

	if strings.Contains(defaults, "type: organization") || strings.Contains(defaults, "resource_type: organization") {
		t.Fatal("resource defaults must not register a second platform organization resource or role template")
	}

	requiredDefaults := []string{
		"- type: zone",
		"- type: group",
		"- type: project",
		"parent_types: [zone]",
		"relations: [zone, owner, admin, developer, operator, viewer]",
	}
	for _, token := range requiredDefaults {
		if !strings.Contains(defaults, token) {
			t.Fatalf("resource defaults are missing required project/zone contract %q", token)
		}
	}
}

func mustReadContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read contract file %s: %v", path, err)
	}
	return string(data)
}
