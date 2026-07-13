from pathlib import Path
import re


def read(path: str) -> str:
    return Path(path).read_text()


def write(path: str, text: str) -> None:
    Path(path).write_text(text)


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"{label}: expected text not found: {old[:180]!r}")
    return text.replace(old, new, 1)


def replace_block(text: str, start: str, end: str, replacement: str, label: str) -> str:
    begin = text.find(start)
    if begin < 0:
        raise SystemExit(f"{label}: start marker not found: {start!r}")
    finish = text.find(end, begin + len(start))
    if finish < 0:
        raise SystemExit(f"{label}: end marker not found: {end!r}")
    return text[:begin] + replacement + text[finish:]


def regex_once(text: str, pattern: str, replacement: str, label: str) -> str:
    out, count = re.subn(pattern, replacement, text, count=1, flags=re.S)
    if count != 1:
        raise SystemExit(f"{label}: expected one regex replacement, got {count}: {pattern[:180]!r}")
    return out


# The duplicate legacy implementation is obsolete even if an earlier branch
# commit still contains it.
Path("internal/biz/project/service.go").unlink(missing_ok=True)

# ---------------------------------------------------------------------------
# Project API: Casdoor Organization is not an Aisphere control-plane object.
# Project.org_id remains a read-only Casdoor identity-domain identifier.
# ---------------------------------------------------------------------------
path = "api/iam/project/v1/project.proto"
text = read(path)
text = replace_once(
    text,
    "// ProjectService manages Aisphere business organizations, projects/workspaces\n// and capability switches. Casdoor organizations remain identity-side\n// projections and are referenced by casdoor_org.\nservice ProjectService {\n",
    "// ProjectService manages projects/workspaces and capability switches.\n// Casdoor Organization is the single identity-domain root; IAM never creates\n// a second Organization. Every project is scoped to Principal.org_id.\nservice ProjectService {\n",
    "project service comment",
)
text = replace_block(
    text,
    "  rpc CreateOrganization",
    "  rpc CreateProject",
    "",
    "organization RPCs",
)
text = replace_once(
    text,
    'option (google.api.http) = { post: "/v1/iam/control-plane/orgs/{org_id}/projects" body: "*" };',
    'option (google.api.http) = { post: "/v1/iam/control-plane/projects" body: "*" };',
    "create project route",
)
text = replace_once(
    text,
    'authz: { action: "create_project" resource: "organization:{org_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "create_project" resource: "iam:project" audience: "iam-service" mode: CHECK_ONLY }',
    "create project access resource",
)
text = replace_block(text, "message Organization {", "message Project {", "", "organization message")
new_create_project = '''message CreateProjectRequest {
  reserved 1, 9;
  reserved "org_id", "owner";

  string slug = 2 [
    (google.api.field_behavior) = REQUIRED,
    (buf.validate.field).string.min_len = 1
  ];
  string display_name = 3 [(google.api.field_behavior) = REQUIRED];
  string description = 4;
  ProjectVisibility visibility = 5;
  map<string, string> labels = 6;
  map<string, string> annotations = 7;
  google.protobuf.Struct metadata = 8;
  repeated string enable_capabilities = 10;
}

'''
text = replace_block(
    text,
    "message CreateOrganizationRequest {",
    "message GetProjectRequest {",
    new_create_project,
    "organization request messages",
)
new_list_projects = '''message ListProjectsRequest {
  reserved 1;
  reserved "org_id";

  string query = 2;
  bool joined = 3;
  LifecycleStatus status = 4;
  ProjectVisibility visibility = 5;
  map<string, string> labels = 6;
  int32 page_size = 7;
  string page_token = 8;
}

'''
text = replace_block(text, "message ListProjectsRequest {", "message ListProjectsReply {", new_list_projects, "list projects request")
write(path, text)

# ---------------------------------------------------------------------------
# Project transport: derive domain and owner from Kernel Principal.
# ---------------------------------------------------------------------------
path = "internal/service/control_plane.go"
text = read(path)
text = replace_block(
    text,
    "func (s *ProjectService) CreateOrganization",
    "func (s *ProjectService) CreateProject",
    "",
    "project organization handlers",
)
create_project = '''func (s *ProjectService) CreateProject(ctx context.Context, req *projectv1.CreateProjectRequest) (*projectv1.Project, error) {
	orgID, actor, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	project, _, err := s.biz.CreateProject(ctx, projectbiz.CreateProjectRequest{
		ZoneID: orgID, Slug: req.GetSlug(), DisplayName: req.GetDisplayName(), Description: req.GetDescription(),
		Visibility: visibilityToStatus(req.GetVisibility()), LabelsJSON: mapStringToJSON(req.GetLabels()), AnnotationsJSON: mapStringToJSON(req.GetAnnotations()),
		CreatedBy: actor, Owner: actor,
	})
	if err != nil {
		return nil, err
	}
	return projectModelToProto(project), nil
}

'''
text = replace_block(text, "func (s *ProjectService) CreateProject", "func (s *ProjectService) GetProject", create_project, "create project handler")
list_projects = '''func (s *ProjectService) ListProjects(ctx context.Context, req *projectv1.ListProjectsRequest) (*projectv1.ListProjectsReply, error) {
	orgID, _, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	page, err := s.repo.ListProjects(ctx, data.ListOptions{OrgID: orgID, Q: req.GetQuery(), Status: lifecycleToStatus(req.GetStatus()), Page: pageFromToken(req.GetPageToken()), Size: int(req.GetPageSize())})
	if err != nil {
		return nil, err
	}
	out := make([]*projectv1.Project, 0, len(page.Items))
	for i := range page.Items {
		out = append(out, projectModelToProto(&page.Items[i]))
	}
	return &projectv1.ListProjectsReply{Projects: out, TotalSize: page.Total, NextPageToken: nextPage(page)}, nil
}

'''
text = replace_block(text, "func (s *ProjectService) ListProjects", "func (s *ProjectService) UpdateProject", list_projects, "list projects handler")
text = replace_block(text, "func organizationModelToProto", "func projectModelToProto", "", "organization proto converter")
text = replace_block(text, "func projectSubject(", "func resourceSubject(", "", "project request subject converter")
project_context = '''func currentProjectContext(ctx context.Context) (string, projectbiz.SubjectRef, error) {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || !principal.IsAuthenticated() {
		return "", projectbiz.SubjectRef{}, authn.ErrMissingCredential("kernel principal is required")
	}
	orgID := strings.TrimSpace(principal.OrgID)
	if orgID == "" {
		return "", projectbiz.SubjectRef{}, authn.ErrMissingCredential("kernel principal org_id is required")
	}
	subjectType := strings.TrimSpace(principal.SubjectType)
	if subjectType == "" {
		subjectType = authn.SubjectTypeUser
	}
	return orgID, projectbiz.SubjectRef{Type: subjectType, ID: strings.TrimSpace(principal.SubjectID)}, nil
}

func currentProjectSubject(ctx context.Context) (projectbiz.SubjectRef, error) {
	_, subject, err := currentProjectContext(ctx)
	return subject, err
}

'''
text = replace_block(text, "func currentProjectSubject", "func currentResourceSubject", project_context, "project principal helper")
text = replace_block(text, "func projectSubjectOr", "func resourceSubjectOr", "", "project owner override helper")
write(path, text)

# Delete the old access bypass for CreateOrganization, preserving directory
# GetOrganization because that is a read-only Casdoor directory operation.
path = "internal/server/access.go"
text = read(path)
text = regex_once(
    text,
    r'\n\s*switch op \{\n\s*case "CreateOrganization", "iam\.project\.v1\.ProjectService/CreateOrganization", "/iam\.project\.v1\.ProjectService/CreateOrganization":\n\s*return accessx\.SkipAuthz\n\s*\}\n',
    '\n',
    "CreateOrganization skip rule",
)
write(path, text)

# ---------------------------------------------------------------------------
# Persistence: remove the second Organization fact source and table.
# ---------------------------------------------------------------------------
path = "internal/data/resource_models.go"
text = read(path)
text = replace_block(text, "type OrganizationModel struct {", "type ProjectModel struct {", "", "OrganizationModel")
text = regex_once(text, r'\n\s*&OrganizationModel\{\},', '', "OrganizationModel migration")
write(path, text)

path = "internal/data/resource_repository.go"
text = read(path)
text = replace_block(text, "\tCreateOrganization", "\tCreateProject", "", "repository Organization interface")
text = replace_block(
    text,
    "func (r *DBControlPlaneRepository) CreateOrganization",
    "func (r *DBControlPlaneRepository) CreateProject",
    "",
    "database Organization repository",
)
write(path, text)

path = "internal/data/memory.go"
text = read(path)
text = regex_once(text, r'\n\s*orgs\s+map\[string\]\*OrganizationModel', '', "memory Organization field")
text = replace_once(
    text,
    "\t\torgs: map[string]*OrganizationModel{}, projects: map[string]*ProjectModel{}, caps: map[string]*CapabilityModel{}, projectCaps: map[string]*ProjectCapabilityModel{},",
    "\t\tprojects: map[string]*ProjectModel{}, caps: map[string]*CapabilityModel{}, projectCaps: map[string]*ProjectCapabilityModel{},",
    "memory Organization initialization",
)
text = replace_block(
    text,
    "func (r *MemoryControlPlaneRepository) CreateOrganization",
    "func (r *MemoryControlPlaneRepository) CreateProject",
    "",
    "memory Organization repository",
)
write(path, text)

# Resource and Grant accept zone as the virtual root and reject the removed
# organization resource type explicitly.
path = "internal/biz/resource/service.go"
text = read(path)
text = replace_once(
    text,
    '''\tcase "organization":
\t\t_, err := s.repo.GetOrganization(ctx, ref.ID)
\t\treturn err
\tcase "project":''',
    '''\tcase "organization":
\t\treturn errors.New("resource type organization is removed; use zone")
\tcase "zone", "group":
\t\tif strings.TrimSpace(ref.ID) == "" {
\t\t\treturn errors.New("resource id is required")
\t\t}
\t\treturn nil
\tcase "project":''',
    "resource root resolver",
)
write(path, text)

path = "internal/biz/grant/service.go"
text = read(path)
text = replace_once(
    text,
    '''\tcase "organization":
\t\treturn s.repo.GetOrganization(ctx, ref.ID)
\tcase "project":''',
    '''\tcase "organization":
\t\treturn nil, errors.New("resource type organization is removed; use zone")
\tcase "project":''',
    "grant root resolver",
)
write(path, text)

path = "docs/envoy-casdoor-oidc.md"
if Path(path).exists():
    write(path, read(path).replace("CreateOrganization.Owner = ctx Principal\n", ""))

# ---------------------------------------------------------------------------
# Contract test: prevent reintroduction of the deleted platform entity.
# ---------------------------------------------------------------------------
path = "internal/biz/project/model_contract_test.go"
text = read(path)
if "TestLegacyOrganizationSurfaceRemoved" not in text:
    marker = "\nfunc mustReadContractFile"
    test = r'''

func TestLegacyOrganizationSurfaceRemoved(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "..")
	files := map[string]string{
		"project proto": mustReadContractFile(t, filepath.Join(root, "api", "iam", "project", "v1", "project.proto")),
		"project transport": mustReadContractFile(t, filepath.Join(root, "internal", "service", "control_plane.go")),
		"project business": mustReadContractFile(t, filepath.Join(root, "internal", "biz", "project", "project.go")),
		"control-plane models": mustReadContractFile(t, filepath.Join(root, "internal", "data", "resource_models.go")),
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
'''
    text = replace_once(text, marker, test + marker, "contract test insertion")
    write(path, text)
