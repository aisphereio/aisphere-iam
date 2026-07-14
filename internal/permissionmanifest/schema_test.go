package permissionmanifest

import (
	"slices"
	"testing"
)

func TestCompareSchemasClassifiesStrictAdditions(t *testing.T) {
	current := mustParseSchema(t, `
definition user {}
definition zone {
  relation owner: user
}`)
	desired := mustParseSchema(t, `
definition user {}
definition zone {
  relation owner: user
  permission view = owner
}`)

	diff := CompareSchemas(current, desired)
	if len(diff.Conflicts) != 0 {
		t.Fatalf("conflicts = %v", diff.Conflicts)
	}
	if !slices.Contains(diff.Additions, "zone.permission.view") {
		t.Fatalf("additions = %v", diff.Additions)
	}
}

func TestCompareSchemasRejectsChangedPermission(t *testing.T) {
	current := mustParseSchema(t, `
definition user {}
definition zone {
  relation owner: user
  permission view = owner
}`)
	desired := mustParseSchema(t, `
definition user {}
definition zone {
  relation owner: user
  permission view = owner + admin
}`)

	diff := CompareSchemas(current, desired)
	if !slices.Contains(diff.Conflicts, "zone.permission.view changed") {
		t.Fatalf("conflicts = %v", diff.Conflicts)
	}
}

func TestCompareSchemasRejectsActiveOnlyDefinition(t *testing.T) {
	current := mustParseSchema(t, `definition user {}
definition legacy {}`)
	desired := mustParseSchema(t, `definition user {}`)

	diff := CompareSchemas(current, desired)
	if !slices.Contains(diff.Conflicts, "definition legacy exists only in active schema") {
		t.Fatalf("conflicts = %v", diff.Conflicts)
	}
}

func TestCompareSchemasIgnoresCommentsAndWhitespace(t *testing.T) {
	current := mustParseSchema(t, `
// current schema
definition user {}
definition zone {
  relation owner:user
  permission view=owner
}`)
	desired := mustParseSchema(t, `
definition user { /* marker */ }
definition zone {
  relation owner: user
  permission view = owner
}`)

	diff := CompareSchemas(current, desired)
	if !diff.Identical() {
		t.Fatalf("diff = %#v", diff)
	}
}

func TestParseSchemaRejectsDuplicateMembers(t *testing.T) {
	_, err := ParseSchema(`definition zone {
  relation owner: user
  relation owner: group
}`)
	if err == nil {
		t.Fatal("expected duplicate member error")
	}
}

func mustParseSchema(t *testing.T, text string) Schema {
	t.Helper()
	schema, err := ParseSchema(text)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}
