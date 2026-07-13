package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIAMP0APIBoundaries(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	iamProto := readAPISurfaceContract(t, filepath.Join(root, "api", "iam", "v1", "iam.proto"))
	identityProto := readAPISurfaceContract(t, filepath.Join(root, "api", "iam", "v1", "identity_admin.proto"))

	directory := protoServiceBlock(t, iamProto, "IAMDirectoryService")
	for _, forbidden := range []string{
		"rpc CreateGroup(",
		"rpc UpdateGroup(",
		"rpc DeleteGroup(",
		"rpc AssignUserToGroup(",
		"rpc RemoveUserFromGroup(",
	} {
		if strings.Contains(directory, forbidden) {
			t.Fatalf("directory read service contains write RPC %q", forbidden)
		}
	}
	for _, required := range []string{"rpc GetUser(", "rpc ListUsers(", "rpc GetOrganization(", "rpc ListGroups(", "rpc GetGroup("} {
		if !strings.Contains(directory, required) {
			t.Fatalf("directory service is missing read RPC %q", required)
		}
	}
	if strings.Contains(directory, `resource: "iam:org:`) {
		t.Fatal("directory policies still reference removed iam:org authorization resources")
	}

	permission := protoServiceBlock(t, iamProto, "IAMPermissionService")
	for _, forbidden := range []string{"rpc WriteRelationship(", "rpc DeleteRelationship("} {
		if strings.Contains(permission, forbidden) {
			t.Fatalf("runtime permission service exposes singular raw tuple RPC %q", forbidden)
		}
	}
	for _, required := range []string{"rpc WriteRelationships(", "rpc DeleteRelationships(", "rpc ReadRelationships("} {
		if !strings.Contains(permission, required) {
			t.Fatalf("runtime permission service is missing internal projection RPC %q", required)
		}
	}

	identityAdmin := protoServiceBlock(t, identityProto, "IAMIdentityAdminService")
	for _, required := range []string{
		"rpc CreateGroup(",
		"rpc UpdateGroup(",
		"rpc DeleteGroup(",
		"rpc AssignUserToGroup(",
		"rpc RemoveUserFromGroup(",
		`resource: "group:{org_id}/{group_id}"`,
	} {
		if !strings.Contains(identityAdmin, required) {
			t.Fatalf("identity admin is missing canonical group write contract %q", required)
		}
	}
}

func readAPISurfaceContract(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func protoServiceBlock(t *testing.T, source, service string) string {
	t.Helper()
	marker := "service " + service + " {"
	start := strings.Index(source, marker)
	if start < 0 {
		t.Fatalf("service %s not found", service)
	}
	depth := 0
	for offset, r := range source[start:] {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start : start+offset+1]
			}
		}
	}
	t.Fatalf("service %s has no closing brace", service)
	return ""
}
