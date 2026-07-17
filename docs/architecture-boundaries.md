# IAM 模块边界与不变量

本文档是 Aisphere IAM 的架构契约。代码、Proto、数据库、SpiceDB Schema 和前端必须遵守这些边界；发生冲突时，先修改架构决策并补充测试，再修改实现。

## 1. 领域边界

### 1.1 Casdoor：身份目录事实源

Casdoor 负责：

- Organization：登录身份域和租户根；
- User：人员身份；
- Group：当前 Organization 下的多级组织树；
- Application、Provider、OIDC 登录配置。

IAM 只做目录适配和授权投影，不复制维护第二套用户、Organization 或 Group 事实。

### 1.2 IAM：控制面与授权编排

IAM 负责：

- 将一个 Casdoor Organization 映射为一个只读 `zone` 根资源；
- 将 Casdoor Group 投影为 SpiceDB `group`；
- 管理 Project、Capability、Resource Type、Role Template 和 Grant；
- 将控制面事实可靠投影到 SpiceDB；
- 为 Hub、Runtime 等业务服务提供权限检查。

IAM 不负责浏览器 Session、OIDC code exchange 和刷新令牌。

### 1.3 SpiceDB：授权查询投影

SpiceDB 保存权限计算所需关系，不作为业务元数据事实源：

```text
zone:<casdoor-org>
group:<group-id>#zone@zone:<casdoor-org>
group:<child>#parent@group:<parent>
project:<project-id>#zone@zone:<casdoor-org>
project:<project-id>#owner@user:<creator-id>
```

不存在第二套 `organization:<id>` 资源。

## 2. 统一资源树

```text
Casdoor Organization
└── zone（内部 1:1 映射，前端显示为“Casdoor 组织”）
    ├── group（多级）
    └── project
        ├── skill_space / skill（skill 同时是 Git 仓库和唯一授权资源）
        ├── agent_space / agent
        ├── tool_space / tool
        ├── sandbox_space / sandbox
        └── runtime_environment / deployment
```

## 3. 强制不变量

### 身份域不变量

- `Principal.org_id` 是当前 Casdoor Organization 的稳定标识；
- 客户端不能通过请求体覆盖当前身份域；
- 路径中的 Org 仅用于目录 API，并必须与 Principal 的可访问域一致。

### Project 不变量

- Project 必须且只能属于一个 `zone`；
- CreateProject 从认证 Principal 获取 Zone，不能接受 `organization_id`；
- 创建者必须自动成为 Project Owner；
- Project 数据库事实与 SpiceDB 结构关系必须由同一个控制面操作产生；
- 投影失败必须可重试、可观测，不允许静默成功。

### Group 不变量

- Group 的事实源是 Casdoor；
- IAM 不维护独立 Group 树；
- Group 作为授权主体时固定使用 `group:<id>#member`；
- Group 结构投影必须包含 `zone`，子组额外包含 `parent`。

### 授权不变量

- Grant 是高层控制面事实；Relationship 是 SpiceDB 查询投影；
- Skill 名称就是 Git 仓库名称；Git clone/fetch/push、评审、合并和发布均检查同一个 `skill:<name>`，不得再创建 `git_repository` 权限资源；
- `main` 是正式发布分支；普通分支需要 `skill#edit`，合并到 `main` 或创建发布 tag 需要 `skill#publish`；
- 普通业务页面不直接暴露 Tuple、relation key 或 UUID；
- Schema 发布、授权、撤销和投影修复必须审计；
- 权限依赖不可用时默认 fail-closed。

## 4. 模块职责

| 模块 | 负责 | 不负责 |
|---|---|---|
| `internal/biz/directory` | Casdoor 目录适配、Group 投影编排 | Project、业务资源 |
| `internal/biz/project` | Project 和 Capability 用例 | Casdoor Organization CRUD |
| `internal/biz/resource` | 通用资源类型与资源投影 | 用户与 Group 事实 |
| `internal/biz/grant` | Role Template、Grant 生命周期 | 直接编辑业务元数据 |
| `internal/biz/projection` | Outbox、重试、投影状态 | 领域规则决策 |
| `internal/data` | PostgreSQL 持久化和事务 | 业务授权决策 |
| `internal/service` | Proto 转换、Principal 提取、错误映射 | 领域规则实现 |

## 5. API 设计规则

- 对外 API contract-first；
- CreateProject 的作用域来自 Principal；
- ListProjects 默认限定当前 Principal 的 Zone；
- 管理接口必须有明确 access policy 和 audit event；
- 不保留已废弃 Organization API 的兼容别名；
- 错误必须区分参数错误、未认证、无权限、冲突、依赖失败和投影失败。

## 6. 破坏性重构策略

平台尚未上线，本轮不提供旧 Platform Organization 数据迁移。部署新模型时：

1. 清空 IAM 测试数据库；
2. 清空 SpiceDB 数据并安装新 Schema；
3. 启动默认目录与管理员 Bootstrap；
4. 重新创建 Group、Project 和测试资源；
5. 执行空库验收。

禁止为旧 `organization_id`、`organization:*` Relationship 或旧 Organization CRUD 增加长期兼容层。
