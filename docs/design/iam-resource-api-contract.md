# IAM Resource Control Plane API Contract（Phase 2）

## 1. 目的

Phase 2 的目标是冻结 `ProjectService`、`ResourceService`、`GrantService` 三组 API contract，先保证业务边界、HTTP/gRPC 入口、access policy 和默认 bootstrap 数据清楚，再进入数据层和服务层实现。

本阶段只新增 proto 和配置，不强行生成 pb 文件。生成动作在具备完整 `buf/protoc/kernel codegen` 工具链后执行。

## 2. API 包

```text
api/iam/project/v1/project.proto
api/iam/resource/v1/resource.proto
api/iam/grant/v1/grant.proto
```

## 3. 路由前缀

为了避免和当前身份目录 API 冲突，资源控制面统一使用：

```text
/v1/iam/control-plane/...
```

已有身份目录接口仍然保留：

```text
/v1/iam/orgs/{org_id}/users
/v1/iam/orgs/{org_id}/groups
/v1/iam/orgs/{org_id}
```

两者语义区别：

| 路由域 | 语义 | 数据来源 |
|---|---|---|
| `/v1/iam/orgs/...` | 身份目录 / Casdoor 投影 | Casdoor / authn adapter |
| `/v1/iam/control-plane/...` | Aisphere 业务资源控制面 | IAM DB + SpiceDB projection |

## 4. ProjectService

负责业务租户和项目治理：

```text
CreateOrganization
GetOrganization
ListOrganizations
UpdateOrganization
CreateProject
GetProject
ListProjects
UpdateProject
ArchiveProject
RegisterCapability
ListCapabilities
EnableProjectCapability
DisableProjectCapability
ListProjectCapabilities
```

### 关键语义

- `Organization` 是 Aisphere 业务组织，不等于 Casdoor Organization。
- `casdoor_org` 是身份侧绑定字段。
- `Project` 是资源协作边界。
- `ProjectCapability` 对应 Erda 的 `ProjectFunction`，但改造成 Aisphere 能力开关。

## 5. ResourceService

负责资源类型、资源投影和资源关系：

```text
RegisterResourceType
GetResourceType
ListResourceTypes
UpsertResource
GetResource
ListResources
MoveResource
ArchiveResource
DeleteResource
BindResource
UnbindResource
ListResourceBindings
BindExternalResource
ListExternalResourceBindings
```

### 关键语义

- `Resource` 是授权投影，不是业务资源本体。
- `owner_service` 仍拥有业务资源本体。
- `ResourceBinding` 表达跨域关系，例如：

```text
agent:a1 --uses_skill--> skill:s1
deployment:d1 --runs_agent--> agent:a1
```

Skill 本身就是 Git 仓库授权对象，仓库名等于 Skill 名；不再维护
`skill -> git_repository` 的映射或第二套权限。

- `ExternalResourceBinding` 用于 Forgejo/Gitea/GitLab/K8s 等外部资源映射。

## 6. GrantService

负责产品层 RBAC 和 SpiceDB 投影：

```text
RegisterRoleTemplate
ListRoleTemplates
GrantAccess
RevokeAccess
ListGrants
ExplainAccess
```

### 关键语义

- `RoleTemplate` 是产品层角色模板。
- `Grant` 是 IAM 的持久化管理事实。
- SpiceDB relationship 是 query-optimized projection。
- 不允许只写 SpiceDB 而不写 `Grant`。

## 7. 默认 bootstrap 数据

新增：

```text
configs/resource/defaults.yaml
```

包含：

- 默认 capabilities：hub、agent、tools、sandbox、runtime。
- 默认 resource types：skill、agent、tool、sandbox、runtime_environment 等。
- 默认 role templates：project developer，以及 skill owner/editor/reviewer/publisher/viewer 等。

Phase 5 会实现 loader/reconciler，将该文件写入 IAM DB，并确保 SpiceDB schema/relationship 投影一致。

## 8. 和 Erda 的对应关系

| Erda | Aisphere Phase 2 |
|---|---|
| Project | Project |
| ProjectFunction | ProjectCapability |
| Application | Capability Space / Resource |
| Member(scopeType, scopeId, roles) | Grant(resource, subject, relation) |
| Project ResourceConfig | ProjectCapability quota/config |

## 9. 下一阶段输入

Phase 3 需要基于本 API contract 设计并实现：

```text
iam_organizations
iam_projects
iam_capabilities
iam_project_capabilities
iam_resource_types
iam_resources
iam_resource_bindings
iam_external_resource_bindings
iam_role_templates
iam_grants
iam_grant_audits
iam_outbox_events
```
