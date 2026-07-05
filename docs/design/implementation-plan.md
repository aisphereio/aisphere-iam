# Aisphere IAM Resource Control Plane 开发改造计划

## Phase 0：设计冻结

交付物：

- `docs/design/iam-resource-control-plane.md`
- `docs/design/iam-rbac-rebac.md`
- `configs/spicedb/aisphere.schema.zed`

验收点：

- kernel / iam / Casdoor / SpiceDB / 业务服务边界明确。
- RBAC / ReBAC 分层明确。
- Resource / Grant / Project 数据模型明确。
- Erda 只迁模型，不迁 legacy 权限实现。

## Phase 1：kernel 增加 resourcex contract

交付物：

- `kernel/resourcex`
  - `ResourceType`
  - `Resource`
  - `ResourceBinding`
  - `ExternalResourceBinding`
  - `RoleTemplate`
  - `Grant`
  - `ResourceRegistry`
  - `GrantManager`
  - memory store for tests/demo

验收点：

- `resourcex` 不依赖 Aisphere 业务概念。
- `resourcex` 可以被 IAM、Hub、Git 等服务作为稳定 contract 引用。

## Phase 2：IAM API 扩展（已完成 contract，待 codegen）

新增 proto：

- `api/iam/project/v1/project.proto`
- `api/iam/resource/v1/resource.proto`
- `api/iam/grant/v1/grant.proto`

新增文档/配置：

- `docs/design/iam-resource-api-contract.md`
- `configs/resource/defaults.yaml`

验收点：

- API contract 已按 `ProjectService / ResourceService / GrantService` 拆分。
- HTTP 路由统一使用 `/v1/iam/control-plane/...`，避免和身份目录 API 冲突。
- 默认 access policy 已在 proto 中声明。
- 默认 capability / resource_type / role_template 已通过 YAML 固化。
- `make api` 和生成 pb 文件需要在完整 codegen 工具链就绪后执行。

## Phase 3：IAM 数据层（进行中）

新增表：

- `iam_organizations`
- `iam_projects`
- `iam_capabilities`
- `iam_project_capabilities`
- `iam_resource_types`
- `iam_resources`
- `iam_resource_bindings`
- `iam_external_resource_bindings`
- `iam_role_templates`
- `iam_grants`
- `iam_grant_audits`
- `iam_outbox_events`

验收点：

- AutoMigrate 本地可跑。
- 关键唯一索引和软删除策略明确。

## Phase 4：IAM biz/service 实现

实现：

- ProjectService
- ResourceService
- GrantService

核心流程：

- CreateProject -> DB + SpiceDB `project#parent` + `project#owner`
- UpsertResource -> DB + SpiceDB `resource#parent`
- BindResource -> DB + SpiceDB cross-resource relation
- GrantAccess -> DB grant + SpiceDB relationship
- RevokeAccess -> DB revoke + SpiceDB relationship delete

## Phase 5：默认注册和 reconciliation

实现：

- 默认 capability 注册。
- 默认 resource_type 注册。
- 默认 role_template 注册。
- 默认 SpiceDB schema 安装。
- IAM grant -> SpiceDB relationship reconcile。

## Phase 6：业务接入

Hub：

- CreateSkill -> UpsertResource(skill)
- BindSkillRepo -> BindResource(skill, backing_repo, repository)
- Edit/Publish -> CheckPermission(skill, edit/publish)

Git：

- CreateRepo -> UpsertResource(git_repository)
- Push -> CheckPermission(repository, write)

Agent/Sandbox/Runtime 后续接入。

## Phase 7：管理前端

第一版功能：

- 组织列表 / 项目列表。
- 项目能力开关。
- 用户/组浏览。
- 项目成员管理。
- 资源列表。
- 资源成员管理。
- 授权记录。
- 权限解释。


### Phase 3 update

Data-layer design has been added in `docs/design/iam-resource-data-layer.md`. The implementation reuses `kernel/dbx` and `kernel/migrationx`; IAM does not introduce a separate database stack.


## Phase 4 update: biz/service layer

Implemented in this batch:

- `internal/biz/project`: organization/project/capability use cases.
- `internal/biz/resource`: resource type, resource projection and binding use cases.
- `internal/biz/grant`: role template, grant, revoke and explain use cases.
- `internal/biz/graph`: provider-neutral relationship projection through `kernel/authz`.
- `internal/biz/defaults`: idempotent loader for `configs/resource/defaults.yaml`.
- `control_plane.defaults` config block for optional default reconciliation.

The implementation deliberately avoids generated protobuf types until the local
`make api` toolchain is available. Generated API handlers should adapt proto
requests into these domain service requests instead of duplicating business
logic.
