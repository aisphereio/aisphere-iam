# Aisphere IAM

Aisphere IAM 是基于 `github.com/aisphereio/kernel` 的**身份目录适配、权限控制面与项目资源控制面服务**。它封装 Casdoor（身份目录）和 SpiceDB（授权查询投影），为 Hub、Runtime 等业务组件提供统一 IAM API。

## 职责边界

```text
Casdoor
├── Organization：登录身份域和租户根
├── User：人员身份
├── Group：Organization 下的多级组织树
└── Application / Provider / OIDC 配置

Envoy Gateway
└── 平台唯一外部 AuthN 边界
    ├── OIDC 登录
    ├── JWT 验证
    └── trusted claim headers

IAM
├── Casdoor 目录适配
├── Casdoor Org → zone 的 1:1 授权投影
├── Group 结构与成员授权投影
├── Project / Capability / Resource Type / Grant 控制面
└── SpiceDB ReBAC 权限检查与投影治理

SpiceDB
└── 权限查询投影，不是业务元数据事实源
```

> **IAM 不负责浏览器 OAuth code exchange、refresh token 或 session 管理。** 这些由 Envoy Gateway 的 OIDC SecurityPolicy 处理。

## 唯一平台根模型

Casdoor Organization 是唯一的顶层身份域。IAM 不维护第二套 Platform Organization。

```text
Casdoor Organization
└── zone（IAM/SpiceDB 内部 1:1 映射）
    ├── group（Casdoor 多级 Group 投影）
    └── project（IAM 业务资源根）
        └── skill / agent / git / sandbox / runtime ...
```

关键不变量：

- `Principal.org_id` 决定当前 Casdoor Organization / Zone；
- 客户端不能覆盖 Project 的 Organization 作用域；
- Project 必须且只能关联一个 Zone；
- Project 创建者自动成为 Owner；
- PostgreSQL 保存控制面事实，SpiceDB 保存关系投影；
- 投影失败必须可重试、可观测，权限检查默认 fail-closed。

完整架构契约见 [`docs/architecture-boundaries.md`](docs/architecture-boundaries.md)。

## 认证模式

IAM 默认使用 `gateway_trusted` 模式，信任 Envoy Gateway 清洗并注入的 `x-aisphere-external-*` headers：

```yaml
security:
  authn:
    enabled: true
    mode: gateway_trusted
```

Kernel middleware 自动从可信 headers 恢复 `authn.Principal` 并注入 Context。业务代码通过 `authn.PrincipalFromContext(ctx)` 获取身份，不能从普通外部 Header 或请求体构造 Principal。

## 主要服务

| 服务 | 说明 |
|---|---|
| `IAMDirectoryService` | Casdoor 用户、Organization 和多级 Group 目录适配 |
| `IAMDirectoryProjectionService` | Group/成员关系投影、重试和漂移检查 |
| `IAMPermissionService` | 内部权限检查、Relationship 投影和资源/主体查找 |
| `IAMAuthorizationAdminService` | Schema、Relationship 和权限诊断管理 |
| `ProjectService` | Project 与 Capability 控制面；不管理第二套 Organization |
| `ResourceService` | 通用业务资源控制面投影 |
| `GrantService` | Role Template 和高层授权 Grant |

## 开发门禁

修改 Proto、授权模型或控制面边界后必须执行：

```bash
make api
make proto-check
make test
make build
```

模型一致性测试会阻止重新引入：

- SpiceDB `definition organization`；
- `project -> organization` 关系；
- `organization` Resource Type 或 Role Template。

## 本地运行

```bash
go run ./cmd/aisphere-iam -conf ./configs/config.local.yaml
```

默认端口：

- HTTP: `0.0.0.0:18080`
- gRPC: `0.0.0.0:19080`
- Metrics: `127.0.0.1:19180`

## 文档

- [`docs/architecture-boundaries.md`](docs/architecture-boundaries.md) — IAM 领域边界与强制不变量
- [`docs/envoy-casdoor-oidc.md`](docs/envoy-casdoor-oidc.md) — Envoy Gateway + Casdoor OIDC 接入
- [`docs/deploy.md`](docs/deploy.md) — 部署方案
- [`docs/kernel-compliance.md`](docs/kernel-compliance.md) — Kernel 合规性
- [`docs/authz-bootstrap-and-permission-console.md`](docs/authz-bootstrap-and-permission-console.md) — 授权引导与权限控制台

## 依赖

- `github.com/aisphereio/kernel`
- Casdoor
- SpiceDB
- PostgreSQL
- Envoy Gateway
