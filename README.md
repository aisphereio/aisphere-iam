# Aisphere IAM

Aisphere IAM 是基于 `github.com/aisphereio/kernel` 的身份认证、目录查询、资源控制面和权限关系服务。它封装 Casdoor 和 SpiceDB，为 Hub、Gateway、Runtime 等业务组件提供统一 IAM API。

## 当前 Authn 阶段

当前阶段先实现 OIDC-only Gateway 接入：

```text
Casdoor OIDC Provider
  -> Envoy Gateway OIDC/JWT/claimToHeaders
  -> upstream services read x-aisphere-external-*
  -> services call IAM when they need user/org/project/permission mapping
```

在 Gateway-trusted 模式下，用户登录态由 Gateway 完成 OIDC/JWT 校验，IAM 后端主要读取 Gateway 注入的可信 Principal headers 并恢复 `ctx.Principal`。因此，bootstrap admin 通过 Casdoor M2M 查询用户 UUID 时不依赖 JWKS。

IAM 只有在自己直接校验 Casdoor token 时才需要 `discovery_url` / `jwks_url`，例如 `VerifyToken`、`casdoor_jwt` / `oidc_jwt` 模式、未来 IAM ExternalAuth token 校验或本地绕过 Gateway 的调试场景。`jwt_certificate_file` 仅作为本地开发或静态兜底使用。IAM 中配置的 `issuer` 必须与 Casdoor token 的 `iss` 完全一致。

详细说明见：

- [`docs/envoy-casdoor-oidc.md`](docs/envoy-casdoor-oidc.md)
- [`docs/deploy.md`](docs/deploy.md)
- [`docs/kernel-compliance.md`](docs/kernel-compliance.md)
- [`docs/authz-bootstrap-and-permission-console.md`](docs/authz-bootstrap-and-permission-console.md)

## Kernel 版本

当前目标 Kernel 版本：`github.com/aisphereio/kernel v0.4.0`。

```bash
make tools
make api
make deploy
make proto-check
make test
make run
```

## 本地运行

```bash
go run ./cmd/aisphere-iam -conf ./configs/config.local.yaml
```

默认端口：

- HTTP: `0.0.0.0:18080`
- gRPC: `0.0.0.0:19080`
- Metrics: `127.0.0.1:19180`

## 主要服务

| 服务 | 说明 |
|---|---|
| `IAMAuthService` | 登录、令牌、用户信息相关能力 |
| `IAMDirectoryService` | 用户、组织、组目录查询 |
| `IAMPermissionService` | 权限检查、关系写入和资源/主体查找 |
| `ProjectService` | 组织、项目、能力开关 |
| `ResourceService` | 资源控制面投影 |
| `GrantService` | 角色模板和授权 Grant |

## 依赖

- `github.com/aisphereio/kernel`
- Casdoor
- SpiceDB
- PostgreSQL
- Gateway API / Envoy Gateway
