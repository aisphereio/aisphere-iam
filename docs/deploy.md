# IAM 部署方案

IAM 运行时只负责服务本身。Gateway API 路由清单由 proto 中的 `google.api.http` 和 `aisphere.access.v1.policy` 生成，`config.yaml` 不再维护公开/认证/内部路由列表，也不再向 etcd 写运行时路由事实。

## 事实源

| 内容 | 事实源 |
|---|---|
| HTTP/gRPC API | `api/**.proto` |
| 路由暴露等级 | `aisphere.access.v1.policy.exposure` |
| Gateway API YAML | `make deploy` 生成到 `deploy/generated` |
| 服务 Deployment/Service/ConfigMap | `deploy/*.yaml` |
| 公开/认证/内部路由列表 | 生成产物，不写入 `configs/config*.yaml` |

## Gateway API ExternalAuth

IAM 不再假设平台自研 Gateway 已经完成认证，也不再依赖 Gateway 注入 `X-Aisphere-*` Principal headers。

推荐链路：

```text
Client
  -> Gateway API / Envoy ExternalAuth
  -> IAM /internal/iam/ext-authz validates Authorization: Bearer <token>
  -> upstream service receives the original Authorization header
  -> upstream service verifies the JWT with oidc_jwt/casdoor_jwt mode
```

`/internal/iam/ext-authz` 是 allow/deny 边界：

- `200`：token 有效，允许 Gateway 继续转发。
- `401`：token 缺失或无效。
- 不向上游注入 `X-Aisphere-*` 身份头。
- 上游服务仍应保留 `Authorization` header，并使用 `security.authn.mode: oidc_jwt` 或 `casdoor_jwt` 自行恢复 Principal。

`gateway_trusted` 只作为旧链路兼容模式，不作为 Gateway API ExternalAuth 主线。

## 生成与部署

```bash
make tools
make api
make deploy
make deploy-apply
```

`make deploy-apply` 会执行：

```bash
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -R -f deploy/generated
```

也可以分开执行：

```bash
make deploy
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -R -f deploy/generated
```

## 前置条件

- 集群已安装 Gateway API CRD。
- `aisphere-system` namespace 下存在生成器配置中引用的 Gateway：
  - `public-gateway`
  - `authenticated-gateway`
  - `internal-gateway`
- Gateway ExternalAuth / ext-authz 指向 IAM 的 `http://aisphere-iam.aisphere:18080/internal/iam/ext-authz`。
- Gateway 转发到上游服务时保留 `Authorization` header。
- `aisphere` namespace 下存在镜像拉取 Secret：`aliyun-registry`。
- PostgreSQL、Casdoor、SpiceDB 服务地址和凭据已按环境替换。

## 配置原则

`configs/config.yaml` 和 `deploy/configmap.yaml` 只保留运行时依赖配置，例如数据库、Casdoor、SpiceDB、metrics、audit。

不在 IAM 配置中维护这些内容：

- public route list
- authenticated route list
- internal route list
- gateway route registry
- etcd route prefix
- Gateway-injected Principal header allowlist

这些路由信息由 proto contract 和 Kernel v0.2.5 的生成器产出，部署时应用 `deploy/generated` 即可。

## 安全注意

仓库内的部署 ConfigMap 使用 `CHANGE_ME_*` 占位符。生产环境应通过平台 Secret 管理或环境专用 overlay 注入真实凭据，不应把真实数据库密码、Casdoor client secret、SpiceDB token 提交到仓库。

Deployment 已移除旧的 privileged sysctl initContainer，并默认启用：

- `runAsNonRoot`
- `allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true`
- `capabilities.drop: [ALL]`
- `seccompProfile: RuntimeDefault`
