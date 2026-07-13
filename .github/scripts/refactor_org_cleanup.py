from pathlib import Path
import re


def replace(path: str, old: str, new: str, count: int = 1) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"expected text not found in {path}: {old[:120]!r}")
    p.write_text(text.replace(old, new, count))


def sub(path: str, pattern: str, repl: str, count: int = 1) -> None:
    p = Path(path)
    text = p.read_text()
    text2, n = re.subn(pattern, repl, text, count=count, flags=re.S)
    if n != count:
        raise SystemExit(f"expected {count} replacement(s) in {path}, got {n}: {pattern[:120]!r}")
    p.write_text(text2)


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
replace(proto, '  string org_id = 2;\n', '  string zone_id = 2;\n', 1)
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

svc = "internal/service/control_plane.go"
sub(svc, r'func \(s \*ProjectService\) CreateOrganization\(.*?\nfunc \(s \*ProjectService\) CreateProject', 'func (s *ProjectService) CreateProject')
sub(
    svc,
    r'func \(s \*ProjectService\) CreateProject\(ctx context\.Context, req \*projectv1\.CreateProjectRequest\) \(\*projectv1\.Project, error\) \{.*?\n\}',
    '''func (s *ProjectService) CreateProject(ctx context.Context, req *projectv1.CreateProjectRequest) (*projectv1.Project, error) {
	zoneID, actor, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	project, _, err := s.biz.CreateProject(ctx, projectbiz.CreateProjectRequest{
		ZoneID: zoneID, Slug: req.GetSlug(), DisplayName: req.GetDisplayName(), Description: req.GetDescription(),
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
	zoneID, _, err := currentProjectContext(ctx)
	if err != nil {
		return nil, err
	}
	page, err := s.repo.ListProjects(ctx, data.ListOptions{OrgID: zoneID, Q: req.GetQuery(), Status: lifecycleToStatus(req.GetStatus()), Page: pageFromToken(req.GetPageToken()), Size: int(req.GetPageSize())})
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
replace(svc, 'OrgId: in.OrgID', 'ZoneId: in.OrgID', 1)
sub(svc, r'\nfunc projectSubject\(.*?\n\}\nfunc resourceSubject', '\nfunc resourceSubject')
sub(
    svc,
    r'func currentProjectSubject\(ctx context\.Context\) \(projectbiz\.SubjectRef, error\) \{.*?\n\}',
    '''func currentProjectContext(ctx context.Context) (string, projectbiz.SubjectRef, error) {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || !principal.IsAuthenticated() {
		return "", projectbiz.SubjectRef{}, authn.ErrMissingCredential("kernel principal is required")
	}
	zoneID := strings.TrimSpace(principal.OrgID)
	if zoneID == "" {
		return "", projectbiz.SubjectRef{}, authn.ErrMissingCredential("kernel principal org_id is required")
	}
	subjectType := strings.TrimSpace(principal.SubjectType)
	if subjectType == "" {
		subjectType = authn.SubjectTypeUser
	}
	return zoneID, projectbiz.SubjectRef{Type: subjectType, ID: strings.TrimSpace(principal.SubjectID)}, nil
}

func currentProjectSubject(ctx context.Context) (projectbiz.SubjectRef, error) {
	_, subject, err := currentProjectContext(ctx)
	return subject, err
}''',
)
sub(svc, r'\nfunc projectSubjectOr\(.*?\n\}\n\nfunc resourceSubjectOr', '\nfunc resourceSubjectOr')

security = "internal/server/security.go"
sub(security, r'\n\tswitch op \{\n\tcase "CreateOrganization".*?\n\t\}\n', '\n')

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

	for path, content := range map[string]string{
		"project proto": proto,
		"project transport service": serviceSource,
		"project business service": projectSource,
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
		} {
			if strings.Contains(content, token) {
				t.Fatalf("%s still contains removed platform Organization token %q", path, token)
			}
		}
	}

	for _, token := range []string{
		`post: "/v1/iam/control-plane/projects"`,
		`reserved "org_id", "owner"`,
		"string zone_id = 2;",
	} {
		if !strings.Contains(proto, token) {
			t.Fatalf("project proto is missing single-zone contract %q", token)
		}
	}

	for _, token := range []string{
		"ZoneID: zoneID",
		"CreatedBy: actor, Owner: actor",
		"OrgID: zoneID",
	} {
		if !strings.Contains(serviceSource, token) {
			t.Fatalf("project service is missing Principal-bound contract %q", token)
		}
	}
}
'''
    test.write_text(t.replace(marker, extra + marker))
