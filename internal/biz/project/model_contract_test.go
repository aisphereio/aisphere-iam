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

	for _, token := range []string{
		"definition organization",
		"relation parent: organization",
		"organization:",
	} {
		if strings.Contains(schema, token) {
			t.Fatalf("SpiceDB schema contains removed platform-organization model %q", token)
		}
	}

	for _, token := range []string{
		"definition zone",
		"definition group",
		"definition project",
		"relation zone: zone",
	} {
		if !strings.Contains(schema, token) {
			t.Fatalf("SpiceDB schema is missing required model contract %q", token)
		}
	}

	for _, token := range []string{
		"type: organization",
		"resource_type: organization",
		"parent_types: [organization]",
	} {
		if strings.Contains(defaults, token) {
			t.Fatalf("resource defaults contain removed platform-organization contract %q", token)
		}
	}

	for _, token := range []string{
		"- type: zone",
		"- type: group",
		"- type: project",
		"parent_types: [zone]",
		"relations: [zone, owner, admin, developer, operator, viewer, custom_binding]",
	} {
		if !strings.Contains(defaults, token) {
			t.Fatalf("resource defaults are missing required project/zone contract %q", token)
		}
	}

	if strings.Contains(projectSource, "graph.Subject(ResourceTypeOrganization") {
		t.Fatal("project creation still projects a legacy organization relationship")
	}

	for _, token := range []string{
		"ResourceTypeZone    = \"zone\"",
		"RelationZone  = \"zone\"",
		"Relation: RelationZone",
		"graph.Subject(ResourceTypeZone, zoneID, \"\")",
	} {
		if !strings.Contains(projectSource, token) {
			t.Fatalf("project creation is missing required zone projection contract %q", token)
		}
	}
}

func TestLegacyOrganizationSurfaceRemoved(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "..")
	files := map[string]string{
		"project proto":            mustReadContractFile(t, filepath.Join(root, "api", "iam", "project", "v1", "project.proto")),
		"project transport":        mustReadContractFile(t, filepath.Join(root, "internal", "service", "control_plane.go")),
		"project business":         mustReadContractFile(t, filepath.Join(root, "internal", "biz", "project", "project.go")),
		"control-plane models":     mustReadContractFile(t, filepath.Join(root, "internal", "data", "resource_models.go")),
		"control-plane repository": mustReadContractFile(t, filepath.Join(root, "internal", "data", "resource_repository.go")),
	}
	for file, content := range files {
		for _, token := range []string{
			"CreateOrganization", "UpdateOrganization", "ArchiveOrganization", "ListOrganizations",
			"CreateOrganizationRequest", "OrganizationModel", "ResourceTypeOrganization",
			"organization:{org_id}", "iam_organizations",
		} {
			if strings.Contains(content, token) {
				t.Fatalf("%s still contains removed platform Organization token %q", file, token)
			}
		}
	}

	proto := files["project proto"]
	for _, token := range []string{
		`post: "/v1/iam/control-plane/projects"`,
		`reserved "org_id", "owner"`,
		"string org_id = 2;",
	} {
		if !strings.Contains(proto, token) {
			t.Fatalf("project proto is missing Principal-scoped contract %q", token)
		}
	}

	transport := files["project transport"]
	for _, token := range []string{"ZoneID: orgID", "CreatedBy: actor, Owner: actor", "OrgID: orgID"} {
		if !strings.Contains(transport, token) {
			t.Fatalf("project service is missing Principal-bound contract %q", token)
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
