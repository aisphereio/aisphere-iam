package server

import "testing"

func TestGeneratedCatalogOwnsAllIAMServices(t *testing.T) {
	want := map[string]bool{
		"i.a.m.directory.projection-service": false,
		"i.a.m.authorization.admin-service":  false,
	}

	for _, module := range IAMModules() {
		if _, ok := want[module.ModuleName()]; ok {
			want[module.ModuleName()] = true
		}
	}

	for service, found := range want {
		if !found {
			t.Errorf("generated IAM catalog is missing %s", service)
		}
	}
}

func TestIAMBindingsMatchGeneratedCatalog(t *testing.T) {
	modules := IAMModules()
	bindings := IAMBindings(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	if len(bindings) != len(modules) {
		t.Fatalf("IAM bindings = %d, generated modules = %d", len(bindings), len(modules))
	}
	for i := range modules {
		if got, want := bindings[i].Module.ModuleName(), modules[i].ModuleName(); got != want {
			t.Errorf("binding %d module = %q, want %q", i, got, want)
		}
	}
}
