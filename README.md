# Aisphere IAM

Aisphere IAM 是基于 `github.com/aisphereio/kernel` 的身份认证、目录查询、资源控制面和权限关系服务。它封装 Casdoor（认证）和 SpiceDB（授权），为 Hub、Gateway、Runtime 等业务组件提供统一 IAM API。

本仓库不是 Kernel layout 模板仓库。开发规则见 `AGENTS.md`，本地运行见 `docs/run-local.md`，部署方案见 `docs/deploy.md`，Kernel 合规说明见 `docs/kernel-compliance.md`。

## 架构

```text
外部请求
  -> Gateway API / Gateway Controller
  -> aisphere-iam HTTP / gRPC server
  -> IAMAuthService        登录、令牌管理、用户信息
  -> IAMDirectoryService   用户、组织、组目录查询
  -> IAMPermissionService  权限检查、关系写入、资源/主体查找
  -> ProjectService        组织、项目、能力开关
  -> ResourceService       资源控制面投影
  -> GrantService          角色模板和授权 Grant
  -> Casdoor               认证后端
  -> SpiceDB               授权后端
```

## Kernel 版本

当前目标 Kernel 版本：`github.com/aisphereio/kernel v0.2.5`。

工具链默认安装 v0.2.5：

```bash
make tools
```

本地开发 Kernel generator 时：

```bash
make tools-local KERNEL_LOCAL=../kernel
```

## 快速开始

```bash
make tools
make api
make deploy
make proto-check
make test
make run
```

## 部署

IAM 部署分两类清单：

| 类型 | 来源 |
|---|---|
| Deployment / Service / ConfigMap / Namespace | `deploy/*.yaml` |
| Gateway API HTTPRoute 等生成清单 | `make deploy` 生成到 `deploy/generated` |

一键生成并部署：

```bash
make deploy-apply
```

等价于：

```bash
make deploy
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -R -f deploy/generated
```

路由事实源是 proto，不是 `config.yaml`。公开、认证、内部路由由 `api/**.proto` 的 `aisphere.access.v1.policy` 和 Kernel deploy generator 决定。

## 本地运行

```bash
go run ./cmd/aisphere-iam -conf ./configs/config.local.yaml
```

默认端口：

- HTTP: `0.0.0.0:18080`
- gRPC: `0.0.0.0:19080`
- Metrics: `127.0.0.1:19180`

## Layout

```text
cmd/aisphere-iam/      Application entrypoint
configs/               Runtime config files
internal/conf/         Config DTOs scanned by configx
internal/data/         Kernel resource initialization
internal/server/       Kernel HTTP and gRPC server construction
internal/service/      IAM 业务服务
deploy/                K8s service manifests
deploy/generated/      Generated Gateway API manifests, created by make deploy
```

## 提供的服务

### IAMAuthService

| 方法 | 说明 |
|------|------|
| `BuildLoginURL` | 构建 Casdoor 登录 URL |
| `ExchangeCode` | 用 code 交换 token |
| `RefreshToken` | 刷新令牌 |
| `VerifyToken` | 验证令牌 |
| `RevokeToken` | 撤销令牌 |
| `GetMe` | 获取当前用户信息 |
| `UpdateMe` | 更新当前用户信息 |
| `GetUserPreferences` | 获取用户偏好 |
| `UpdateUserPreferences` | 更新用户偏好 |

### IAMDirectoryService

| 方法 | 说明 |
|------|------|
| `GetUser` | 获取用户 |
| `ListUsers` | 列出用户 |
| `GetOrganization` | 获取组织 |
| `ListGroups` | 列出组 |

### IAMPermissionService

| 方法 | 说明 |
|------|------|
| `CheckPermission` | 检查权限 |
| `WriteRelationship` | 写入关系 |
| `DeleteRelationship` | 删除关系 |
| `LookupResources` | 查找资源 |
| `LookupSubjects` | 查找主体 |

## 验证

```bash
curl http://127.0.0.1:18080/healthz
curl http://127.0.0.1:18080/readyz
curl "http://127.0.0.1:18080/v1/iam/login-url?redirect_uri=http://localhost:3001/auth/callback&state=/"
```

## 依赖

- `github.com/aisphereio/kernel` — 核心框架
- Casdoor — 身份认证
- SpiceDB — 关系授权
- PostgreSQL — IAM 控制面存储
- Gateway API — Kubernetes 路由发布
