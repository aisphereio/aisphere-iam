# IAM 本地运行

推荐目录结构：

```text
E:\coding\aisphereio\
  kernel\
  aisphere-iam\
  aisphere-gateway\
```

如果 Kernel generator 有本地改动，先安装本地工具链：

```powershell
cd E:\coding\aisphereio\aisphere-iam
make tools-local KERNEL_LOCAL=..\kernel
make api
make proto-check
make test
```

如果只使用已发布 Kernel 版本：

```powershell
make tools
make api
make proto-check
make test
```

启动前确认外部依赖：

- Casdoor：`security.authn.enabled=true` 时必须可访问。
- SpiceDB：`security.authz.enabled=true` 且 `dev_allow_all=false` 时必须可访问。
- etcd：`gateway.route_registry.provider=etcd` 时必须可访问，用于向 Gateway 注册 route manifest。

默认端口：

```text
IAM HTTP:    0.0.0.0:18080
IAM gRPC:    0.0.0.0:19080
IAM metrics: 127.0.0.1:19180
```

## 重要：配置文件选择

IAM 有两个配置文件，**必须使用正确的配置启动**：

| 文件 | 用途 | client_secret | application_name |
|------|------|--------------|-----------------|
| `configs/config.yaml` | 默认配置（生产/CI） | `${CASDOOR_CLIENT_SECRET}`（环境变量） | `aisphere` |
| `configs/config.local.yaml` | 本地开发配置 | 硬编码值 | `aisphere-iam` |

**本地开发必须使用 `config.local.yaml`**，因为 `config.yaml` 中的 `client_secret` 是环境变量引用，如果未设置会导致 Casdoor 连接失败。

## 本地启动

### 方式一：使用 config.local.yaml（推荐）

```powershell
# 设置环境变量
$env:POSTGRES_DSN="postgres://postgres:ChangeMe_PostgreSQL_123@36.137.200.194:30080/aisphere_iam?sslmode=disable"
$env:SPICEDB_TOKEN="keykeykey"

# 启动（必须指定 -conf 参数）
go run ./cmd/aisphere-iam -conf ./configs/config.local.yaml
```

### 方式二：使用 make run（加载 config.yaml）

```powershell
$env:CASDOOR_CLIENT_SECRET="6d37fc7a95c21c45e543207704345b2ac80586d2"
$env:SPICEDB_TOKEN="keykeykey"
$env:POSTGRES_DSN="postgres://postgres:ChangeMe_PostgreSQL_123@36.137.200.194:30080/aisphere_iam?sslmode=disable"
make run
```

## 外部依赖凭据

| 服务 | 端点 | 凭据 |
|------|------|------|
| PostgreSQL | `36.137.200.194:30080` | 用户: `postgres`, 密码: `ChangeMe_PostgreSQL_123`, 数据库: `aisphere_iam` |
| SpiceDB | `36.137.200.194:30084` | Preshared Key: `keykeykey` |
| Casdoor | `36.137.200.194:30082` | client_id: `3319f6430828f3b2bd8f`, client_secret: `6d37fc7a95c21c45e543207704345b2ac80586d2`, app: `aisphere-iam` |
| etcd | `36.137.200.194:30086` | 无需认证 |

## 验证

```powershell
# 健康检查
curl http://127.0.0.1:18080/healthz

# 获取登录 URL（IAM 前端回调地址）
curl "http://127.0.0.1:18080/v1/iam/login-url?redirect_uri=http://localhost:3001/auth/callback&state=/"

# 列出本地用户
curl http://127.0.0.1:18080/v1/users
```

## 常见问题

### Casdoor code exchange 返回 500

检查 `config.local.yaml` 中的 Casdoor 配置：
- `application_name` 必须是 Casdoor 中配置的 app 名称（如 `aisphere-iam`）
- `client_id` 和 `client_secret` 必须与 Casdoor app 配置一致
- Casdoor 的 app 配置中必须设置正确的 `redirect_uri`（如 `http://localhost:3001/auth/callback`）

### /v1/iam/me 返回 403

这是 SpiceDB 权限拒绝。`GetMe` 端点不需要 SpiceDB 检查，如果出现此错误，检查 `internal/server/access.go` 中的 `iamAccessResolver` 是否正确跳过了 `GetMe` 操作。

### 数据库连接失败

确认 PostgreSQL 连接字符串正确：
```
postgres://postgres:ChangeMe_PostgreSQL_123@36.137.200.194:30080/aisphere_iam?sslmode=disable
```

生产部署时，IAM direct HTTP 端口应仅在内网开放；公网入口统一走 Gateway。
