package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunValidatesCommittedPermissionManifest(t *testing.T) {
	root := filepath.Join("..", "..")
	var output bytes.Buffer
	err := run([]string{
		"--manifest", filepath.Join(root, "configs", "resource", "defaults.yaml"),
		"--schema", filepath.Join(root, "configs", "spicedb", "aisphere.schema.zed"),
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	want := "permission manifest valid: 15 resource types, 23 role templates, 24 schema definitions"
	if !strings.Contains(output.String(), want) {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}
