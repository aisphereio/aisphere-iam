# Aisphere IAM

Aisphere IAM 是基于 `github.com/aisphereio/kernel` 的身份认证、目录查询和权限关系服务。它封装 Casdoor（认证）和 SpiceDB（授权），为 Hub、Gateway、Runtime 等业务组件提供统一 IAM API。

本仓库不是 Kernel layout 模板仓库。开发规则见 `AGENTS.md`，本地运行见 `docs/run-local.md`，Kernel 合规说明见 `docs/kernel-compliance.md`。

## 架构

```text
外部请求
  -> HTTP / gRPC server
  -> IAMAuthService   (登录、令牌管理、用户信息)
  -> IAMDirectoryService (用户、组织、组目录查询)
  -> IAMPermissionService (权限检查、关系写入、资源/主体查找)
  -> Casdoor (认证后端)
  -> SpiceDB (授权后端)
```

## 快速开始

```powershell
# 安装工具链（使用本地 kernel 开发时）
make tools-local KERNEL_LOCAL=../kernel

# 生成 API 代码
make api

# 检查 proto
make proto-check

# 运行测试
make test

# 启动服务
make run
```

## 本地运行

```powershell
go run ./cmd/aisphere-iam -conf ./configs/config.local.yaml
```

默认端口：

- HTTP: `0.0.0.0:18080`
- gRPC: `0.0.0.0:19080`
- Metrics: `127.0.0.1:19180`

## Layout

```text
cmd/aisphere-iam/      Application entrypoint
configs/               Local config files
internal/conf/         Config DTOs scanned by configx
internal/data/         Kernel resource initialization (Casdoor, SpiceDB, etc.)
internal/registry/     Route registry client (etcd)
internal/server/       Kernel HTTP and gRPC server construction
internal/service/      IAM 业务服务
  ├── authn.go         IAMAuthService — 登录、令牌、用户信息
  ├── directory.go    IAMDirectoryService — 用户/目录查询
  └── permission.go   IAMPermissionService — 权限检查/关系管理
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
# 健康检查
curl http://127.0.0.1:18080/healthz

# 获取登录 URL
curl http://127.0.0.1:18080/v1/iam/login-url
```

## 依赖

- `github.com/aisphereio/kernel` — 核心框架
- Casdoor — 身份认证
- SpiceDB — 关系授权
- etcd — route registry 存储（可选）