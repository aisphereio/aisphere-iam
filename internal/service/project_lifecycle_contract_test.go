package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectLifecycleContractHasNoUnimplementedHandlers(t *testing.T) {
	root := filepath.Join("..", "..")
	source, err := os.ReadFile(filepath.Join(root, "internal", "service", "control_plane.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, forbidden := range []string{"UpdateProject is not implemented", "ArchiveProject is not implemented"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("project lifecycle stub returned: %s", forbidden)
		}
	}
	proto, err := os.ReadFile(filepath.Join(root, "api", "iam", "project", "v1", "project.proto"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proto), "google.protobuf.FieldMask update_mask") {
		t.Fatal("UpdateProjectRequest must define update_mask")
	}
}
