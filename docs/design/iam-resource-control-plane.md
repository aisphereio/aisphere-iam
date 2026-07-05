# Aisphere IAM Resource Control Plane 设计

## 1. 目标

Aisphere 当前已经有：

- Casdoor：AuthN / User / Group / Organization / Application / OIDC。
- SpiceDB：AuthZ / Relationship / Permission Check / Lookup。
- aisphere-iam：Auth facade、Directory facade、Permission facade。

现在缺失的是位于 Casdoor 和 SpiceDB 中间的资源控制面：

```text
Casdoor = 人和登录
Resource Control Plane = 有哪些业务资源、资源属于哪个项目、资源之间是什么关系、授权怎么管理
SpiceDB = 能不能访问的权限计算图
```

该模块第一阶段放在 `aisphere-iam` 内部实现，代码边界保持独立，后续可以拆成 `aisphere-resource` 服务。

## 2. 核心边界

### kernel 负责

`kernel` 只提供 provider-neutral contract：

- `authn`：认证、token、identity directory 抽象，以及 Casdoor adapter。
- `authz`：ObjectRef、SubjectRef、Relationship、Check、Lookup，以及 SpiceDB adapter。
- `resourcex`：ResourceType、Resource、ResourceBinding、RoleTemplate、Grant 及接口。

`kernel` 不放 Aisphere 业务资源实现，不出现 Skill、Repo、Agent、Sandbox 等具体业务概念。

### aisphere-iam 负责

`aisphere-iam` 实现平台控制面：

- Organization / Project / Capability。
- ResourceType / Resource / ResourceBinding / ExternalResourceBinding。
- RoleTemplate / Grant / Revoke / Audit / Outbox。
- Grant 到 SpiceDB Relationship 的投影。
- IAM 管理前端需要的查询 API。

### 业务服务负责

- Hub：Skill / SkillSpace 本体。
- Git：Repository / GitNamespace 本体。
- Agent：Agent 本体。
- Sandbox：Sandbox 本体。
- Runtime：Deployment / RuntimeEnvironment 本体。

业务服务创建或变更资源时，向 IAM 注册资源投影和资源关系。

## 3. Erda 迁移原则

Erda 的价值是领域模型，不是直接迁 legacy 代码。

可吸收：

- `organization -> project -> application` 三层模型。
- Project 作为资源治理边界，而不是简单目录。
- ProjectFunction 改造成 ProjectCapability。
- Member 的 `scope_type + scope_id + roles` 改造成 `resource_ref + subject_ref + relation`。
- ProjectDTO 中 owners、joined、can_manage、stats、labels 的产品模型。

不迁移：

- Erda legacy endpoint。
- Erda bundle/dao/permission adaptor。
- Erda DOP application、pipeline、runtime 业务耦合。
- Erda 自身 permission handler。继承和权限推导交给 SpiceDB。

## 4. 资源树

第一版资源树：

```text
Organization
└── Project / Workspace
    ├── SkillSpace
    │   └── Skill
    ├── GitNamespace
    │   └── Repository
    ├── AgentSpace
    │   └── Agent
    ├── ToolSpace
    │   └── Tool
    ├── SandboxSpace
    │   └── Sandbox
    └── RuntimeEnvironment
        └── Deployment
```

暂不做独立授权：

- git_commit
- git_file
- skill_file
- skill_version
- branch
- run_log

这些默认继承父资源权限。

## 5. 数据模型

### organizations

```sql
create table iam_organizations (
  id text primary key,
  slug text unique not null,
  display_name text not null,
  status text not null,
  casdoor_org text,
  plan text,
  region text,
  metadata_json jsonb,
  created_at timestamptz not null,
  updated_at timestamptz not null
);
```

### projects

```sql
create table iam_projects (
  id text primary key,
  org_id text not null,
  slug text not null,
  display_name text not null,
  description text,
  status text not null,
  visibility text not null default 'private',
  labels_json jsonb,
  annotations_json jsonb,
  created_by text,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  unique(org_id, slug)
);
```

### capabilities / project_capabilities

```sql
create table iam_capabilities (
  id text primary key,
  name text unique not null,
  display_name text not null,
  owner_service text not null,
  status text not null,
  config_schema jsonb
);

create table iam_project_capabilities (
  project_id text not null,
  capability_id text not null,
  enabled bool not null default true,
  config_json jsonb,
  quota_json jsonb,
  primary key(project_id, capability_id)
);
```

内置 capability：

- hub
- git
- agent
- tools
- sandbox
- runtime

### resource_types

```sql
create table iam_resource_types (
  type text primary key,
  capability_id text not null,
  owner_service text not null,
  parent_types jsonb,
  grantable bool not null,
  auditable bool not null,
  spicedb_type text not null,
  relations_json jsonb,
  permissions_json jsonb,
  metadata_schema jsonb,
  status text not null
);
```

### resources

```sql
create table iam_resources (
  id text primary key,
  type text not null,
  org_id text not null,
  project_id text,
  parent_type text,
  parent_id text,
  owner_service text not null,
  owner_resource_id text not null,
  slug text,
  display_name text,
  path text,
  status text not null,
  visibility text not null default 'private',
  labels_json jsonb,
  annotations_json jsonb,
  metadata_json jsonb,
  created_by text,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  deleted_at timestamptz
);
```

### resource_bindings

```sql
create table iam_resource_bindings (
  id text primary key,
  source_type text not null,
  source_id text not null,
  relation text not null,
  target_type text not null,
  target_id text not null,
  status text not null,
  created_by text,
  created_at timestamptz not null,
  unique(source_type, source_id, relation, target_type, target_id)
);
```

示例：

```text
skill:s1 --backing_repo--> git_repository:r1
agent:a1 --uses_skill--> skill:s1
agent:a1 --uses_tool--> tool:t1
deployment:d1 --runs_agent--> agent:a1
```

### role_templates

```sql
create table iam_role_templates (
  id text primary key,
  resource_type text not null,
  role_key text not null,
  display_name text not null,
  description text,
  relation text not null,
  built_in bool not null,
  enabled bool not null,
  sort_order int,
  metadata_json jsonb,
  unique(resource_type, role_key)
);
```

### grants

```sql
create table iam_grants (
  id text primary key,
  resource_type text not null,
  resource_id text not null,
  role_key text not null,
  relation text not null,
  subject_type text not null,
  subject_id text not null,
  subject_relation text,
  source text not null,
  reason text,
  expires_at timestamptz,
  created_by_type text,
  created_by_id text,
  created_at timestamptz not null,
  revoked_at timestamptz,
  consistency_token text
);
```

## 6. API 分包

建议新增：

```text
api/iam/project/v1/project.proto
api/iam/resource/v1/resource.proto
api/iam/grant/v1/grant.proto
```

第一版服务：

- ProjectService
  - CreateOrganization
  - GetOrganization
  - ListOrganizations
  - CreateProject
  - GetProject
  - ListProjects
  - UpdateProject
  - ArchiveProject
  - EnableProjectCapability
  - DisableProjectCapability
  - ListProjectCapabilities

- ResourceService
  - RegisterResourceType
  - GetResourceType
  - ListResourceTypes
  - UpsertResource
  - GetResource
  - ListResources
  - MoveResource
  - ArchiveResource
  - DeleteResource
  - BindResource
  - UnbindResource
  - ListResourceBindings

- GrantService
  - RegisterRoleTemplate
  - ListRoleTemplates
  - GrantAccess
  - RevokeAccess
  - ListGrants
  - ExplainAccess

## 7. 开发顺序

1. 冻结设计文档。
2. 在 kernel 增加 `resourcex` neutral contract。
3. 在 IAM 增加 API proto 草案。
4. 在 IAM 增加 DB model 和 migration。
5. 实现 Project / Resource / Grant biz 和 service。
6. 实现 Grant -> SpiceDB Relationship 投影。
7. 注册默认 ResourceType、RoleTemplate、SpiceDB schema。
8. 接 Hub/Git 的 CreateSkill/CreateRepo/BindSkillRepo。
9. 做 IAM 管理前端。


## 12. Phase 2 API Contract

Phase 2 新增三组 proto：

```text
api/iam/project/v1/project.proto
api/iam/resource/v1/resource.proto
api/iam/grant/v1/grant.proto
```

控制面 HTTP 路由统一使用：

```text
/v1/iam/control-plane/...
```

这样可以避免和当前 Casdoor 身份目录接口混淆。当前 `/v1/iam/orgs/...` 仍表示身份目录；新的 `/v1/iam/control-plane/orgs/...` 表示 Aisphere 业务组织。

详细 contract 见：

```text
docs/design/iam-resource-api-contract.md
```

默认 capability、resource type、role template 固化在：

```text
configs/resource/defaults.yaml
```

该配置 Phase 5 再接入 loader/reconciler。


## Phase 3 data layer

The persistent control-plane schema is defined in `migrations/000001_iam_resource_control_plane.sql`. Runtime database access reuses `kernel/dbx`; migration execution reuses `kernel/migrationx`. The IAM service exposes a `ControlPlaneRepository` from `internal/data.Resources` for the Phase 4 biz/service layer.


## Phase 4 biz-layer boundary

The control plane now has a first domain-service layer:

```text
ProjectService domain layer
  owns organization/project/capability workflows.

ResourceService domain layer
  owns resource type, projection, binding and external binding workflows.

GrantService domain layer
  owns product-level RBAC role templates and grants.
```

All SpiceDB writes are performed through `kernel/authz.RelationshipWriter`, never
through direct SpiceDB imports. This keeps the earlier boundary intact:

```text
kernel = provider-neutral contract
spicedb adapter = concrete authz engine
aisphere-iam = resource/grant control plane
```
