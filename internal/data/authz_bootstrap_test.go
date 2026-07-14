package data

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/logx"

	"github.com/aisphereio/aisphere-iam/internal/conf"
)

func TestBootstrapAuthzSchemaSkipsIdenticalSchema(t *testing.T) {
	desired := "definition user {}\ndefinition zone {\n  relation owner: user\n}"
	manager := &fakeSchemaManager{current: authz.Schema{Text: desired}}

	err := BootstrapAuthzSchema(context.Background(), schemaConfig(t, desired), manager, logx.Noop())
	if err != nil {
		t.Fatal(err)
	}
	if manager.validateCalls != 0 || manager.writeCalls != 0 {
		t.Fatalf("validate calls = %d, write calls = %d", manager.validateCalls, manager.writeCalls)
	}
}

func TestBootstrapAuthzSchemaPublishesMissingPermission(t *testing.T) {
	current := "definition user {}\ndefinition zone {\n  relation owner: user\n}"
	desired := "definition user {}\ndefinition zone {\n  relation owner: user\n  permission view = owner\n}"
	manager := &fakeSchemaManager{current: authz.Schema{Text: current}}

	err := BootstrapAuthzSchema(context.Background(), schemaConfig(t, desired), manager, logx.Noop())
	if err != nil {
		t.Fatal(err)
	}
	if manager.validateCalls != 1 || manager.writeCalls != 1 {
		t.Fatalf("validate calls = %d, write calls = %d", manager.validateCalls, manager.writeCalls)
	}
	if manager.written.Text != desired {
		t.Fatalf("written schema = %q", manager.written.Text)
	}
}

func TestBootstrapAuthzSchemaRejectsChangedPermission(t *testing.T) {
	current := "definition user {}\ndefinition zone {\n  relation owner: user\n  permission view = owner\n}"
	desired := "definition user {}\ndefinition zone {\n  relation owner: user\n  permission view = owner + admin\n}"
	manager := &fakeSchemaManager{current: authz.Schema{Text: current}}

	err := BootstrapAuthzSchema(context.Background(), schemaConfig(t, desired), manager, logx.Noop())
	if err == nil || !strings.Contains(err.Error(), "zone.permission.view changed") {
		t.Fatalf("error = %v", err)
	}
	if manager.writeCalls != 0 {
		t.Fatalf("write calls = %d", manager.writeCalls)
	}
}

func TestBootstrapAuthzSchemaRejectsActiveOnlyDefinition(t *testing.T) {
	current := "definition user {}\ndefinition legacy {}"
	desired := "definition user {}"
	manager := &fakeSchemaManager{current: authz.Schema{Text: current}}

	err := BootstrapAuthzSchema(context.Background(), schemaConfig(t, desired), manager, logx.Noop())
	if err == nil || !strings.Contains(err.Error(), "definition legacy exists only in active schema") {
		t.Fatalf("error = %v", err)
	}
	if manager.writeCalls != 0 {
		t.Fatalf("write calls = %d", manager.writeCalls)
	}
}

func TestBootstrapAuthzSchemaFailsClosedWhenReadFails(t *testing.T) {
	manager := &fakeSchemaManager{readErr: errors.New("read unavailable")}

	err := BootstrapAuthzSchema(context.Background(), schemaConfig(t, "definition user {}"), manager, logx.Noop())
	if err == nil || !strings.Contains(err.Error(), "read active authz schema failed") {
		t.Fatalf("error = %v", err)
	}
	if manager.writeCalls != 0 {
		t.Fatalf("write calls = %d", manager.writeCalls)
	}
}

func schemaConfig(t *testing.T, desired string) conf.AuthzConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.zed")
	if err := os.WriteFile(path, []byte(desired), 0o600); err != nil {
		t.Fatal(err)
	}
	return conf.AuthzConfig{SchemaPath: path}
}

type fakeSchemaManager struct {
	current       authz.Schema
	readErr       error
	validateErr   error
	writeErr      error
	validateCalls int
	writeCalls    int
	written       authz.Schema
}

func (f *fakeSchemaManager) ReadSchema(context.Context) (authz.Schema, error) {
	return f.current, f.readErr
}

func (f *fakeSchemaManager) ValidateSchema(context.Context, authz.Schema) error {
	f.validateCalls++
	return f.validateErr
}

func (f *fakeSchemaManager) WriteSchema(_ context.Context, schema authz.Schema) error {
	f.writeCalls++
	f.written = schema
	return f.writeErr
}
