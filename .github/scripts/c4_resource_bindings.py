from pathlib import Path
import re
import textwrap


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing expected fragment: {label}")
    return text.replace(old, new, 1)


def remove_braced_block(source: str, marker: str) -> str:
    start = source.find(marker)
    if start < 0:
        raise SystemExit(f"missing block marker: {marker}")
    brace = source.find("{", start)
    if brace < 0:
        raise SystemExit(f"missing opening brace: {marker}")
    depth = 0
    end = None
    for i in range(brace, len(source)):
        if source[i] == "{":
            depth += 1
        elif source[i] == "}":
            depth -= 1
            if depth == 0:
                end = i + 1
                break
    if end is None:
        raise SystemExit(f"missing closing brace: {marker}")
    while end < len(source) and source[end] == "\n":
        end += 1
    return source[:start] + source[end:]


# Remove unsafe, underspecified resource move/delete operations from the release contract.
proto_path = Path("api/iam/resource/v1/resource.proto")
proto = proto_path.read_text()
for marker in [
    "  rpc MoveResource(",
    "  rpc DeleteResource(",
    "message MoveResourceRequest {",
    "message DeleteResourceRequest {",
    "message DeleteResourceReply {",
]:
    proto = remove_braced_block(proto, marker)
proto_path.write_text(proto)

# Extend repository query/filter and binding lifecycle contracts.
repo_path = Path("internal/data/resource_repository.go")
repo = repo_path.read_text()
repo = replace_once(
    repo,
    "\tResourceType string\n\tResourceID   string\n\tSubjectType",
    "\tResourceType string\n\tResourceID   string\n\tTargetType   string\n\tTargetID     string\n\tRelation     string\n\tProvider     string\n\tExternalType string\n\tExternalID   string\n\tSubjectType",
    "ListOptions resource binding filters",
)
repo = replace_once(
    repo,
    "\tBindResource(ctx context.Context, binding *ResourceBindingModel, outbox ...*OutboxEventModel) error\n\tListResourceBindings(ctx context.Context, opts ListOptions) ([]ResourceBindingModel, error)\n\tBindExternalResource(ctx context.Context, binding *ExternalResourceBindingModel) error\n",
    "\tBindResource(ctx context.Context, binding *ResourceBindingModel, outbox ...*OutboxEventModel) error\n\tGetResourceBinding(ctx context.Context, id string) (*ResourceBindingModel, error)\n\tUnbindResource(ctx context.Context, id string, outbox ...*OutboxEventModel) error\n\tListResourceBindings(ctx context.Context, opts ListOptions) (*Page[ResourceBindingModel], error)\n\tBindExternalResource(ctx context.Context, binding *ExternalResourceBindingModel) error\n\tListExternalResourceBindings(ctx context.Context, opts ListOptions) (*Page[ExternalResourceBindingModel], error)\n",
    "ControlPlaneRepository binding lifecycle",
)
old_db_list = '''func (r *DBControlPlaneRepository) ListResourceBindings(ctx context.Context, opts ListOptions) ([]ResourceBindingModel, error) {
\tvar out []ResourceBindingModel
\tquery, args := whereBuilder().eq("source_type", opts.ResourceType).eq("source_id", opts.ResourceID).eq("status", opts.Status).build()
\treturn out, r.db.FindMany(ctx, &out, query, args...)
}
'''
new_db_list = '''func (r *DBControlPlaneRepository) GetResourceBinding(ctx context.Context, id string) (*ResourceBindingModel, error) {
\tvar out ResourceBindingModel
\tif err := r.db.FindOne(ctx, &out, "id = ?", id); err != nil {
\t\treturn nil, err
\t}
\treturn &out, nil
}

func (r *DBControlPlaneRepository) UnbindResource(ctx context.Context, id string, outbox ...*OutboxEventModel) error {
\treturn r.db.InTx(ctx, func(tx dbx.Tx) error {
\t\tif err := tx.Update(ctx, &ResourceBindingModel{}, "id = ?", []any{id}, map[string]any{"status": StatusArchived, "updated_at": time.Now().UTC()}); err != nil {
\t\t\treturn err
\t\t}
\t\treturn createOutbox(ctx, tx, outbox...)
\t})
}

func (r *DBControlPlaneRepository) ListResourceBindings(ctx context.Context, opts ListOptions) (*Page[ResourceBindingModel], error) {
\tvar out []ResourceBindingModel
\tquery, args := whereBuilder().eq("source_type", opts.ResourceType).eq("source_id", opts.ResourceID).eq("target_type", opts.TargetType).eq("target_id", opts.TargetID).eq("relation", opts.Relation).eq("status", opts.Status).build()
\tres, err := r.db.Paginate(ctx, &out, &ResourceBindingModel{}, query, args, opts.Page, opts.Size)
\treturn pageFrom(out, res, err)
}
'''
repo = replace_once(repo, old_db_list, new_db_list, "DB resource binding query")
repo = replace_once(
    repo,
    '''func (r *DBControlPlaneRepository) BindExternalResource(ctx context.Context, binding *ExternalResourceBindingModel) error {
\treturn r.db.SafeUpsert(ctx, binding, []string{"resource_type", "resource_id", "external_path", "external_url", "sync_mode", "sync_status", "last_synced_at", "metadata_json", "updated_at"})
}
''',
    '''func (r *DBControlPlaneRepository) BindExternalResource(ctx context.Context, binding *ExternalResourceBindingModel) error {
\treturn r.db.SafeUpsert(ctx, binding, []string{"resource_type", "resource_id", "external_path", "external_url", "sync_mode", "sync_status", "last_synced_at", "metadata_json", "updated_at"})
}

func (r *DBControlPlaneRepository) ListExternalResourceBindings(ctx context.Context, opts ListOptions) (*Page[ExternalResourceBindingModel], error) {
\tvar out []ExternalResourceBindingModel
\tquery, args := whereBuilder().eq("resource_type", opts.ResourceType).eq("resource_id", opts.ResourceID).eq("provider", opts.Provider).eq("external_type", opts.ExternalType).eq("external_id", opts.ExternalID).eq("sync_status", opts.Status).build()
\tres, err := r.db.Paginate(ctx, &out, &ExternalResourceBindingModel{}, query, args, opts.Page, opts.Size)
\treturn pageFrom(out, res, err)
}
''',
    "DB external binding list",
)
repo_path.write_text(repo)

# Replace memory binding queries and add lifecycle methods.
memory_path = Path("internal/data/memory.go")
memory = memory_path.read_text()
pattern = re.compile(r'func \(r \*MemoryControlPlaneRepository\) ListResourceBindings\(.*?\n\}\n', re.DOTALL)
replacement = '''func (r *MemoryControlPlaneRepository) GetResourceBinding(_ context.Context, id string) (*ResourceBindingModel, error) {
\tr.mu.RLock()
\tdefer r.mu.RUnlock()
\tv := r.bindings[id]
\tif v == nil {
\t\treturn nil, fmt.Errorf("%w: resource binding %s", ErrNotFound, id)
\t}
\treturn clone(v), nil
}

func (r *MemoryControlPlaneRepository) UnbindResource(_ context.Context, id string, events ...*OutboxEventModel) error {
\tr.mu.Lock()
\tdefer r.mu.Unlock()
\tv := r.bindings[id]
\tif v == nil {
\t\treturn fmt.Errorf("%w: resource binding %s", ErrNotFound, id)
\t}
\tv.Status = StatusArchived
\tv.UpdatedAt = time.Now().UTC()
\tfor _, event := range events {
\t\tr.saveEvent(event)
\t}
\treturn nil
}

func (r *MemoryControlPlaneRepository) ListResourceBindings(_ context.Context, opts ListOptions) (*Page[ResourceBindingModel], error) {
\tr.mu.RLock()
\tdefer r.mu.RUnlock()
\tvar out []ResourceBindingModel
\tfor _, v := range r.bindings {
\t\tif (opts.ResourceType == "" || v.SourceType == opts.ResourceType) &&
\t\t\t(opts.ResourceID == "" || v.SourceID == opts.ResourceID) &&
\t\t\t(opts.TargetType == "" || v.TargetType == opts.TargetType) &&
\t\t\t(opts.TargetID == "" || v.TargetID == opts.TargetID) &&
\t\t\t(opts.Relation == "" || v.Relation == opts.Relation) && statusOK(v.Status, opts.Status) {
\t\t\tout = append(out, *clone(v))
\t\t}
\t}
\treturn pageOf(out, opts.Page, opts.Size), nil
}
'''
memory, count = pattern.subn(replacement, memory, count=1)
if count != 1:
    raise SystemExit(f"failed to replace memory binding list: {count}")
insert_after = '''func (r *MemoryControlPlaneRepository) BindExternalResource(ctx context.Context, b *ExternalResourceBindingModel) error {
\tr.mu.Lock()
\tdefer r.mu.Unlock()
\tv := clone(b)
\tv.CreatedAt = nowIfZero(v.CreatedAt)
\tv.UpdatedAt = nowIfZero(v.UpdatedAt)
\tif v.SyncStatus == "" {
\t\tv.SyncStatus = StatusPending
\t}
\tr.externalBindings[v.ID] = v
\treturn nil
}
'''
addition = insert_after + '''
func (r *MemoryControlPlaneRepository) ListExternalResourceBindings(_ context.Context, opts ListOptions) (*Page[ExternalResourceBindingModel], error) {
\tr.mu.RLock()
\tdefer r.mu.RUnlock()
\tvar out []ExternalResourceBindingModel
\tfor _, v := range r.externalBindings {
\t\tif (opts.ResourceType == "" || v.ResourceType == opts.ResourceType) &&
\t\t\t(opts.ResourceID == "" || v.ResourceID == opts.ResourceID) &&
\t\t\t(opts.Provider == "" || v.Provider == opts.Provider) &&
\t\t\t(opts.ExternalType == "" || v.ExternalType == opts.ExternalType) &&
\t\t\t(opts.ExternalID == "" || v.ExternalID == opts.ExternalID) && statusOK(v.SyncStatus, opts.Status) {
\t\t\tout = append(out, *clone(v))
\t\t}
\t}
\treturn pageOf(out, opts.Page, opts.Size), nil
}
'''
memory = replace_once(memory, insert_after, addition, "memory external binding list")
memory_path.write_text(memory)

# Remove unsupported service handlers and old incomplete binding handlers.
control_path = Path("internal/service/control_plane.go")
control = control_path.read_text()
for method in ["MoveResource", "DeleteResource", "UnbindResource", "ListResourceBindings", "ListExternalResourceBindings"]:
    pattern = re.compile(rf'func \(s \*ResourceService\) {method}\(.*?\n\}}\n\n', re.DOTALL)
    control, count = pattern.subn("", control, count=1)
    if count != 1:
        raise SystemExit(f"failed to remove ResourceService.{method}: {count}")
if "status." not in control:
    control = control.replace('\t"google.golang.org/grpc/status"\n', "")
if "codes." not in control:
    control = control.replace('\t"google.golang.org/grpc/codes"\n', "")
control_path.write_text(control)

Path("internal/biz/resource/lifecycle.go").write_text(textwrap.dedent('''\
    package resource

    import (
        "context"
        "errors"
        "strings"

        "github.com/aisphereio/aisphere-iam/internal/data"
        "github.com/aisphereio/kernel/authz"
    )

    type UnbindResourceRequest struct {
        ID string
    }

    func (s *Service) UnbindResource(ctx context.Context, req UnbindResourceRequest) (*data.ResourceBindingModel, authz.WriteResult, error) {
        if s.repo == nil {
            return nil, authz.WriteResult{}, errors.New("resource service repository is nil")
        }
        id := strings.TrimSpace(req.ID)
        if id == "" {
            return nil, authz.WriteResult{}, errors.New("binding_id is required")
        }
        binding, err := s.repo.GetResourceBinding(ctx, id)
        if err != nil {
            return nil, authz.WriteResult{}, err
        }
        if binding.Status == data.StatusArchived {
            return binding, authz.WriteResult{}, nil
        }
        rel, err := s.bindingRelationship(ctx, BindResourceRequest{
            Source: ResourceRef{Type: binding.SourceType, ID: binding.SourceID},
            Relation: binding.Relation,
            Target: ResourceRef{Type: binding.TargetType, ID: binding.TargetID},
        })
        if err != nil {
            return nil, authz.WriteResult{}, err
        }
        filter := authz.RelationshipFilter{
            ResourceType: rel.Resource.Type,
            ResourceID: rel.Resource.ID,
            Relation: rel.Relation,
            SubjectType: rel.Subject.Type,
            SubjectID: rel.Subject.ID,
            SubjectRel: rel.Subject.Relation,
        }
        event, err := s.projection.NewDeleteEvent("resource_binding", binding.ID, filter, rel)
        if err != nil {
            return nil, authz.WriteResult{}, err
        }
        if err := s.repo.UnbindResource(ctx, binding.ID, event); err != nil {
            return nil, authz.WriteResult{}, err
        }
        binding.Status = data.StatusArchived
        wr, err := s.projection.Dispatch(ctx, event)
        return binding, wr, err
    }
'''))

Path("internal/service/resource_binding_lifecycle.go").write_text(textwrap.dedent('''\
    package service

    import (
        "context"

        resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
        resourcebiz "github.com/aisphereio/aisphere-iam/internal/biz/resource"
        "github.com/aisphereio/aisphere-iam/internal/data"
    )

    func (s *ResourceService) UnbindResource(ctx context.Context, req *resourcev1.UnbindResourceRequest) (*resourcev1.UnbindResourceReply, error) {
        binding, _, err := s.biz.UnbindResource(ctx, resourcebiz.UnbindResourceRequest{ID: req.GetBindingId()})
        if err != nil {
            return nil, err
        }
        return &resourcev1.UnbindResourceReply{BindingId: binding.ID, Unbound: binding.Status == data.StatusArchived}, nil
    }

    func (s *ResourceService) ListResourceBindings(ctx context.Context, req *resourcev1.ListResourceBindingsRequest) (*resourcev1.ListResourceBindingsReply, error) {
        source, target := req.GetSource(), req.GetTarget()
        page, err := s.repo.ListResourceBindings(ctx, data.ListOptions{
            ResourceType: source.GetType(), ResourceID: source.GetId(),
            TargetType: target.GetType(), TargetID: target.GetId(), Relation: req.GetRelation(), Status: req.GetStatus(),
            Page: pageFromToken(req.GetPageToken()), Size: int(req.GetPageSize()),
        })
        if err != nil {
            return nil, err
        }
        out := make([]*resourcev1.ResourceBinding, 0, len(page.Items))
        for i := range page.Items {
            out = append(out, resourceBindingModelToProto(&page.Items[i]))
        }
        return &resourcev1.ListResourceBindingsReply{Bindings: out, TotalSize: page.Total, NextPageToken: nextPage(page)}, nil
    }

    func (s *ResourceService) ListExternalResourceBindings(ctx context.Context, req *resourcev1.ListExternalResourceBindingsRequest) (*resourcev1.ListExternalResourceBindingsReply, error) {
        resource := req.GetResource()
        page, err := s.repo.ListExternalResourceBindings(ctx, data.ListOptions{
            ResourceType: resource.GetType(), ResourceID: resource.GetId(), Provider: req.GetProvider(),
            ExternalType: req.GetExternalType(), ExternalID: req.GetExternalId(), Status: req.GetSyncStatus(),
            Page: pageFromToken(req.GetPageToken()), Size: int(req.GetPageSize()),
        })
        if err != nil {
            return nil, err
        }
        out := make([]*resourcev1.ExternalResourceBinding, 0, len(page.Items))
        for i := range page.Items {
            out = append(out, externalBindingModelToProto(&page.Items[i]))
        }
        return &resourcev1.ListExternalResourceBindingsReply{Bindings: out, TotalSize: page.Total, NextPageToken: nextPage(page)}, nil
    }
'''))

Path("internal/biz/resource/lifecycle_test.go").write_text(textwrap.dedent('''\
    package resource

    import (
        "context"
        "testing"

        "github.com/aisphereio/aisphere-iam/internal/data"
        "github.com/aisphereio/kernel/authz"
    )

    func TestResourceBindingLifecycleAndExternalLookup(t *testing.T) {
        ctx := context.Background()
        repo := data.NewMemoryControlPlaneRepository()
        store := authz.NewMemoryRelationshipStore()
        service := NewService(repo, store)

        for _, rt := range []*data.ResourceTypeModel{
            {Type: "skill", SpiceDBType: "skill", RelationsJSON: `["backing_repo"]`, Status: data.StatusActive},
            {Type: "git_repository", SpiceDBType: "git_repository", RelationsJSON: `[]`, Status: data.StatusActive},
        } {
            if err := repo.UpsertResourceType(ctx, rt); err != nil { t.Fatal(err) }
        }
        for _, resource := range []*data.ResourceModel{
            {Type: "skill", ID: "skill-1", OrgID: "zone-a", OwnerService: "hub", OwnerResourceID: "skill-1", Status: data.StatusActive},
            {Type: "git_repository", ID: "repo-1", OrgID: "zone-a", OwnerService: "git", OwnerResourceID: "repo-1", Status: data.StatusActive},
        } {
            if err := repo.UpsertResource(ctx, resource); err != nil { t.Fatal(err) }
        }

        binding, _, err := service.BindResource(ctx, BindResourceRequest{
            ID: "binding-1", Source: ResourceRef{Type: "skill", ID: "skill-1"},
            Relation: RelationBackingRepo, Target: ResourceRef{Type: "git_repository", ID: "repo-1"},
        })
        if err != nil { t.Fatalf("BindResource: %v", err) }
        rels, _ := store.ReadRelationships(ctx, authz.RelationshipFilter{})
        if len(rels) != 1 { t.Fatalf("relationships after bind = %#v", rels) }

        archived, wr, err := service.UnbindResource(ctx, UnbindResourceRequest{ID: binding.ID})
        if err != nil { t.Fatalf("UnbindResource: %v", err) }
        if archived.Status != data.StatusArchived || wr.Deleted != 1 { t.Fatalf("unbind result = (%#v, %#v)", archived, wr) }
        rels, _ = store.ReadRelationships(ctx, authz.RelationshipFilter{})
        if len(rels) != 0 { t.Fatalf("relationships after unbind = %#v", rels) }
        if _, _, err := service.UnbindResource(ctx, UnbindResourceRequest{ID: binding.ID}); err != nil { t.Fatalf("idempotent unbind: %v", err) }

        if _, err := service.BindExternalResource(ctx, BindExternalResourceRequest{
            ID: "external-1", Resource: ResourceRef{Type: "skill", ID: "skill-1"},
            Provider: "gitlab", ExternalType: "project", ExternalID: "42",
        }); err != nil { t.Fatalf("BindExternalResource: %v", err) }
        page, err := repo.ListExternalResourceBindings(ctx, data.ListOptions{Provider: "gitlab", ExternalID: "42"})
        if err != nil || page.Total != 1 || len(page.Items) != 1 { t.Fatalf("external lookup = (%#v, %v)", page, err) }
    }
'''))

Path("internal/service/resource_binding_contract_test.go").write_text(textwrap.dedent('''\
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
        if err != nil { t.Fatal(err) }
        service := protoServiceBlock(t, string(protoBytes), "ResourceService")
        for _, forbidden := range []string{"rpc MoveResource(", "rpc DeleteResource("} {
            if strings.Contains(service, forbidden) { t.Fatalf("underspecified release RPC returned: %s", forbidden) }
        }
        for _, required := range []string{"rpc UnbindResource(", "rpc ListExternalResourceBindings("} {
            if !strings.Contains(service, required) { t.Fatalf("binding lifecycle RPC missing: %s", required) }
        }
        source, err := os.ReadFile(filepath.Join(root, "internal", "service", "control_plane.go"))
        if err != nil { t.Fatal(err) }
        for _, forbidden := range []string{"UnbindResource is not implemented", "ListExternalResourceBindings is not implemented", "MoveResource is not implemented", "DeleteResource is not implemented"} {
            if strings.Contains(string(source), forbidden) { t.Fatalf("resource stub returned: %s", forbidden) }
        }
    }
'''))

Path(".agile-v/change_requests/CR-0004-close-resource-binding-lifecycle.md").write_text(textwrap.dedent('''\
    # CR-0004 — Close Resource Binding Lifecycle

    ## Status

    `IMPLEMENTED_PENDING_VERIFICATION [C4]`

    ## Release decisions

    - implement the complete Bind → List → Unbind lifecycle for internal resource relationships;
    - implement filtered, paginated external-resource binding lookup;
    - remove `MoveResource` and `DeleteResource` from the first release contract until hierarchy, dependent-resource, hard-delete and projection-compensation rules are approved;
    - retain `ArchiveResource` as the supported non-destructive lifecycle operation.

    ## Acceptance criteria

    - Unbind archives the binding fact and deletes the exact projected relationship through the durable outbox path;
    - repeated Unbind is idempotent;
    - binding lists honor source, target, relation, status and pagination filters;
    - external binding lists honor resource/provider/external identity/sync status and pagination filters;
    - generated routes no longer expose Move/Delete operations;
    - no ResourceService RPC in the release contract returns `Unimplemented`;
    - generation, contract checks, all Go tests, build, drift and container build pass.
'''))
