package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourceReleaseContractHasClosedBindingLifecycle(t *testing.T) {
	root := filepath.Join("..", "..")
	protoBytes, err := os.ReadFile(filepath.Join(root, "api", "iam", "resource", "v1", "resource.proto"))
	if err != nil {
		t.Fatal(err)
	}
	service := protoServiceBlock(t, string(protoBytes), "ResourceService")
	for _, forbidden := range []string{"rpc MoveResource(", "rpc DeleteResource("} {
		if strings.Contains(service, forbidden) {
			t.Fatalf("underspecified release RPC returned: %s", forbidden)
		}
	}
	for _, required := range []string{"rpc UnbindResource(", "rpc ListExternalResourceBindings("} {
		if !strings.Contains(service, required) {
			t.Fatalf("binding lifecycle RPC missing: %s", required)
		}
	}
	source, err := os.ReadFile(filepath.Join(root, "internal", "service", "control_plane.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"UnbindResource is not implemented", "ListExternalResourceBindings is not implemented", "MoveResource is not implemented", "DeleteResource is not implemented"} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("resource stub returned: %s", forbidden)
		}
	}
}
