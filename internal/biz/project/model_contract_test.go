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
	projectSource := mustReadContractFile(t, filepath.Join(root, "internal", "biz", "project", "project.go"))

	forbiddenSchema := []string{
		"definition organization",
		"relation parent: organization",
		"organization:",
	}
	for _, token := range forbiddenSchema {
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

	forbiddenDefaults := []string{
		"type: organization",
		"resource_type: organization",
		"parent_types: [organization]",
	}
	for _, token := range forbiddenDefaults {
		if strings.Contains(defaults, token) {
			t.Fatalf("resource defaults contain removed platform-organization contract %q", token)
		}
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

	forbiddenProjectSource := []string{
		"Relation: RelationParent,\n\t\tSubject:  graph.Subject(ResourceTypeOrganization",
	}
	for _, token := range forbiddenProjectSource {
		if strings.Contains(projectSource, token) {
			t.Fatalf("project creation still projects the legacy organization relationship %q", token)
		}
	}

	requiredProjectSource := []string{
		"ResourceTypeZone         = \"zone\"",
		"RelationZone   = \"zone\"",
		"Relation: RelationZone",
		"graph.Subject(ResourceTypeZone, zoneID, \"\")",
	}
	for _, token := range requiredProjectSource {
		if !strings.Contains(projectSource, token) {
			t.Fatalf("project creation is missing required zone projection contract %q", token)
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
