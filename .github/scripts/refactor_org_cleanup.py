from pathlib import Path
import re


def replace(path: str, old: str, new: str, count: int = 1) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"expected text not found in {path}: {old[:160]!r}")
    p.write_text(text.replace(old, new, count))


def sub(path: str, pattern: str, repl: str, count: int = 1) -> None:
    p = Path(path)
    text = p.read_text()
    text2, n = re.subn(pattern, repl, text, count=count, flags=re.S)
    if n != count:
        raise SystemExit(f"expected {count} replacement(s) in {path}, got {n}: {pattern[:160]!r}")
    p.write_text(text2)


# ---------------------------------------------------------------------------
# Public control-plane contract: no IAM-local Organization CRUD. org_id remains
# a read-only identity-domain identifier on Project responses, while create/list
# scope is always derived from Principal.org_id.
# ---------------------------------------------------------------------------
proto = "api/iam/project/v1/project.proto"
sub(
    proto,
    r'// ProjectService manages Aisphere business organizations, projects/workspaces\n// and capability switches\. Casdoor organizations remain identity-side\n// projections and are referenced by casdoor_org\.\nservice ProjectService \{\n',
    '// ProjectService manages projects/workspaces and capability switches.\n'
    '// Casdoor Organization is the single identity-domain root; IAM never creates\n'
    '// a second Organization. Every project is scoped to Principal.org_id.\n'
    'service ProjectService {\n',
)
sub(proto, r'  rpc CreateOrganization\(.*?\n  rpc CreateProject', '  rpc CreateProject')
replace(proto, 'post: "/v1/iam/control-plane/orgs/{org_id}/projects"', 'post: "/v1/iam/control-plane/projects"')
replace(
    proto,
    'authz: { action: "create_project" resource: "organization:{org_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "create_project" resource: "iam:project" audience: "iam-service" mode: CHECK_ONLY }',
)
sub(proto, r'\nmessage Organization \{.*?\n\}\n\nmessage Project \{', '\nmessage Project {')
sub(
    proto,
    r'\nmessage CreateOrganizationRequest \{.*?\nmessage CreateProjectRequest \{.*?\n\}',
    '''
message CreateProjectRequest {
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
}''',
)
sub(
    proto,
    r'\nmessage ListProjectsRequest \{.*?\n\}',
    '''
message ListProjectsRequest {
  reserved 1;
  reserved "org_id";

  string query = 2;
  bool joined = 3;
  LifecycleStatus status = 4;
  ProjectVisibility visibility = 5;
  map<string, string> labels = 6;
  int32 page_size = 7;
  string page_token = 8;
}''',
)

# ---------------------------------------------------------------------------
# Transport: actor and identity domain are authoritative Kernel Principal data.
# ---------------------------------------------------------------------------
svc = "internal/service/control_plane.go"
sub(svc, r'func \(s \*ProjectService\) CreateOrganization\(.*?\nfunc \(s \*ProjectService\) CreateProject', 'func (s *ProjectService) CreateProject')
sub(
    svc,
    r'func \(s \*ProjectService\) CreateProject\(ctx context\.Context, req \*projectv1\.CreateProjectRequest\) \(\*projectv1\.Project, error\) \{.*?\n\}',
    '''func (s *ProjectService) CreateProject(ctx context.Context, req *projectv1.CreateProjectRequest) (*projectv1.Project, error) {
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
}''',
)
sub(
    svc,
    r'func \(s \*ProjectService\) ListProjects\(ctx context\.Context, req \*projectv1\.ListProjectsRequest\) \(\*projectv1\.ListProjectsReply, error\) \{.*?\n\}',
    '''func (s *ProjectService) ListProjects(ctx context.Context, req *projectv1.ListProjectsRequest) (*projectv1.ListProjectsReply, error) {
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
}''',
)
sub(svc, r'\nfunc organizationModelToProto\(.*?\n\}\n\nfunc projectModelToProto', '\nfunc projectModelToProto')
sub(svc, r'\nfunc projectSubject\(.*?\n\}\nfunc resourceSubject', '\nfunc resourceSubject')
sub(
    svc,
    r'func currentProjectSubject\(ctx context\.Context\) \(projectbiz\.SubjectRef, error\) \{.*?\n\}',
    '''func currentProjectContext(ctx context.Context) (string, projectbiz.SubjectRef, error) {
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
}''',
)
sub(svc, r'\nfunc projectSubjectOr\(.*?\n\}\n\nfunc resourceSubjectOr', '\nfunc resourceSubjectOr')

# Remove the obsolete authorization bypass for the deleted operation.
access = "internal/server/access.go"
sub(access, r'\n\tswitch op \{\n\tcase "CreateOrganization".*?\n\t\}\n', '\n')

# ---------------------------------------------------------------------------
# Persistence: delete the second Organization fact source entirely.
# ---------------------------------------------------------------------------
models = "internal/data/resource_models.go"
sub(
    models,
    r'\ntype OrganizationModel \{.*?\nfunc \(OrganizationModel\) TableName\(\) string \{ return "iam_organizations" \}\n',
    '\n',
)
sub(models, r'\n\s*&OrganizationModel\{\},', '', count=1)

repo = "internal/data/resource_repository.go"
sub(
    repo,
    r'\n\tCreateOrganization\(ctx context\.Context, org \*OrganizationModel, outbox \.\.\.\*OutboxEventModel\) error\n\tUpsertOrganization\(ctx context\.Context, org \*OrganizationModel\) error\n\tGetOrganization\(ctx context\.Context, id string\) \(\*OrganizationModel, error\)\n\tListOrganizations\(ctx context\.Context, opts ListOptions\) \(\*Page\[OrganizationModel\], error\)\n\tArchiveOrganization\(ctx context\.Context, id string\) error\n',
    '\n',
)
sub(
    repo,
    r'\nfunc \(r \*DBControlPlaneRepository\) CreateOrganization\(.*?\nfunc \(r \*DBControlPlaneRepository\) CreateProject',
    '\nfunc (r *DBControlPlaneRepository) CreateProject',
)

memory = "internal/data/memory.go"
sub(memory, r'\n\s*orgs\s+map\[string\]\*OrganizationModel', '')
replace(
    memory,
    '\t\torgs: map[string]*OrganizationModel{}, projects: map[string]*ProjectModel{}, caps: map[string]*CapabilityModel{}, projectCaps: map[string]*ProjectCapabilityModel{},',
    '\t\tprojects: map[string]*ProjectModel{}, caps: map[string]*CapabilityModel{}, projectCaps: map[string]*ProjectCapabilityModel{},',
)
sub(
    memory,
    r'\nfunc \(r \*MemoryControlPlaneRepository\) CreateOrganization\(.*?\nfunc \(r \*MemoryControlPlaneRepository\) CreateProject',
    '\nfunc (r *MemoryControlPlaneRepository) CreateProject',
)

# Old resource type must be rejected; zone is a virtual root backed by Casdoor.
resource_service = "internal/biz/resource/service.go"
replace(
    resource_service,
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
)

grant_service = "internal/biz/grant/service.go"
replace(
    grant_service,
    '''\tcase "organization":
\t\treturn s.repo.GetOrganization(ctx, ref.ID)
\tcase "project":''',
    '''\tcase "organization":
\t\treturn nil, errors.New("resource type organization is removed; use zone")
\tcase "project":''',
)

# Remove the obsolete actor rule from the current deployment guide.
envoy_doc = Path("docs/envoy-casdoor-oidc.md")
if envoy_doc.exists():
    text = envoy_doc.read_text()
    text = text.replace("CreateOrganization.Owner = ctx Principal\n", "")
    envoy_doc.write_text(text)

# ---------------------------------------------------------------------------
# Regression contract: the old surface must never be regenerated.
# ---------------------------------------------------------------------------
test = Path("internal/biz/project/model_contract_test.go")
t = test.read_text()
marker = '\nfunc mustReadContractFile'
if 'TestLegacyOrganizationSurfaceRemoved' not in t:
    extra = r'''

func TestLegacyOrganizationSurfaceRemoved(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "..")
	proto := mustReadContractFile(t, filepath.Join(root, "api", "iam", "project", "v1", "project.proto"))
	serviceSource := mustReadContractFile(t, filepath.Join(root, "internal", "service", "control_plane.go"))
	projectSource := mustReadContractFile(t, filepath.Join(root, "internal", "biz", "project", "project.go"))
	models := mustReadContractFile(t, filepath.Join(root, "internal", "data", "resource_models.go"))
	repository := mustReadContractFile(t, filepath.Join(root, "internal", "data", "resource_repository.go"))

	for path, content := range map[string]string{
		"project proto": proto,
		"project transport service": serviceSource,
		"project business service": projectSource,
		"control-plane models": models,
		"control-plane repository": repository,
	} {
		for _, token := range []string{
			"CreateOrganization",
			"UpdateOrganization",
			"ArchiveOrganization",
			"ListOrganizations",
			"CreateOrganizationRequest",
			"OrganizationModel",
			"ResourceTypeOrganization",
			"organization:{org_id}",
			"iam_organizations",
		} {
			if strings.Contains(content, token) {
				t.Fatalf("%s still contains removed platform Organization token %q", path, token)
			}
		}
	}

	for _, token := range []string{
		`post: "/v1/iam/control-plane/projects"`,
		`reserved "org_id", "owner"`,
		"string org_id = 2;",
	} {
		if !strings.Contains(proto, token) {
			t.Fatalf("project proto is missing Principal-scoped contract %q", token)
		}
	}

	for _, token := range []string{
		"ZoneID: orgID",
		"CreatedBy: actor, Owner: actor",
		"OrgID: orgID",
	} {
		if !strings.Contains(serviceSource, token) {
			t.Fatalf("project service is missing Principal-bound contract %q", token)
		}
	}
}
'''
    test.write_text(t.replace(marker, extra + marker))
