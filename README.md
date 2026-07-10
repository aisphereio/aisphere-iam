# Aisphere IAM

Aisphere IAM 是基于 `github.com/aisphereio/kernel` 的**身份目录、权限管理和控制面服务**。它封装 Casdoor（身份目录）和 SpiceDB（授权），为 Hub、Runtime 等业务组件提供统一 IAM API。

## 职责边界

```text
Casdoor        = 外部身份源 / OIDC Provider / 用户与组织目录
Envoy Gateway  = 平台唯一外部 Authn 边界（OIDC 登录、JWT 验证、claimToHeaders）
IAM            = Casdoor 目录适配
               Aisphere 身份投影
               SpiceDB ReBAC 权限控制面
               不再负责浏览器 OAuth 登录流程
```

> **IAM 不再承担浏览器 OAuth code exchange、refresh、session 管理。** 这些由 Envoy Gateway 的 OIDC SecurityPolicy 处理。

## 认证模式

IAM 默认使用 `gateway_trusted` 模式，信任 Envoy Gateway 注入的 `x-aisphere-external-*` headers：

```yaml
security:
  authn:
    enabled: true
    mode: gateway_trusted
```

在此模式下，Kernel middleware 自动从 headers 恢复 `authn.Principal` 并注入 Context。业务代码通过 `authn.PrincipalFromContext(ctx)` 获取。

## 主要服务

| 服务 | 说明 |
|---|---|
| `IAMDirectoryService` | 用户、组织、组目录查询 |
| `IAMPermissionService` | 权限检查、关系写入和资源/主体查找 |
| `ProjectService` | 组织、项目、能力开关 |
| `ResourceService` | 资源控制面投影 |
| `GrantService` | 角色模板和授权 Grant |

## 本地运行

```bash
go run ./cmd/aisphere-iam -conf ./configs/config.local.yaml
```

默认端口：

- HTTP: `0.0.0.0:18080`
- gRPC: `0.0.0.0:19080`
- Metrics: `127.0.0.1:19180`

## 文档

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