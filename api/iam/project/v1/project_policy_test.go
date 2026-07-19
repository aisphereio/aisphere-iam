package projectv1

import "testing"

func TestListProjectsUsesZoneVisibilityPermission(t *testing.T) {
	rule, ok := ProjectServiceKernelAuthzRules["/iam.project.v1.ProjectService/ListProjects"]
	if !ok {
		t.Fatal("ListProjects authorization rule is missing")
	}
	if rule.Action != "view_zone" || rule.Resource != "zone:{org_id}" {
		t.Fatalf("ListProjects rule = %q on %q, want view_zone on zone:{org_id}", rule.Action, rule.Resource)
	}
}
