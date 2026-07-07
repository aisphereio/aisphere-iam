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

IAM 不再依赖平台自研 Gateway 认证链路，但仍保留 Gateway-trusted 身份传递模型：认证由 IAM 完成，身份头由 Envoy Gateway 在 ExternalAuth 成功后注入到上游，业务服务不需要重复校验外部 Casdoor/OIDC JWT。

推荐链路：

```text
Client
  -> Gateway API / Envoy ExternalAuth
  -> IAM ExternalAuthorize validates Authorization: Bearer <external-token>
  -> IAM returns Gateway-controlled X-Aisphere-* headers and X-Aisphere-Internal-Token
  -> Envoy strips any client-supplied X-Aisphere-* headers, then forwards only IAM-returned headers
  -> upstream service runs gateway_trusted authn and restores Principal from trusted headers
```

ExternalAuth 端点来自 proto：

```text
POST /internal/iam/ext-authz -> iam.v1.IAMAuthService/ExternalAuthorize
```

`ExternalAuthorize` 的语义：

- 输入使用请求头 `Authorization: Bearer <external-token>`。
- 成功时返回 `200`，并在 response headers 写入 `X-Aisphere-*` Principal headers。
- 成功时可同时写入 `X-Aisphere-Internal-Token`，后端服务在 `gateway_trusted` 模式下用它校验调用确实来自可信 Gateway。
- 失败时返回认证错误，Gateway 不应继续转发。

Envoy/Gateway 必须先清理外部伪造头：

```text
X-Aisphere-Auth-Verified
X-Aisphere-Subject
X-Aisphere-Subject-Type
X-Aisphere-Provider
X-Aisphere-External-ID
X-Aisphere-Issuer
X-Aisphere-Audience
X-Aisphere-Owner
X-Aisphere-Org-ID
X-Aisphere-App-ID
X-Aisphere-Username
X-Aisphere-Name
X-Aisphere-Email
X-Aisphere-Groups
X-Aisphere-Roles
X-Aisphere-Scopes
X-Aisphere-Internal-Token
```

后端服务默认仍使用：

```yaml
security:
  authn:
    enabled: true
    mode: gateway_trusted
  internal_call:
    enabled: true
    header: X-Aisphere-Internal-Token
    token: CHANGE_ME_INTERNAL_TOKEN
```

`oidc_jwt/casdoor_jwt` 可用于高安全服务的二次校验模式，但不是普通业务服务默认心智负担。

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
- Gateway 转发到上游服务时只保留或注入 IAM 返回的 `X-Aisphere-*` 和 `X-Aisphere-Internal-Token` 可信头。
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

这些路由信息由 proto contract 和 Kernel v0.2.5 的生成器产出，部署时应用 `deploy/generated` 即可。

## 安全注意

仓库内的部署 ConfigMap 使用 `CHANGE_ME_*` 占位符。生产环境应通过平台 Secret 管理或环境专用 overlay 注入真实凭据，不应把真实数据库密码、Casdoor client secret、SpiceDB token 提交到仓库。

Deployment 已移除旧的 privileged sysctl initContainer，并默认启用：

- `runAsNonRoot`
- `allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true`
- `capabilities.drop: [ALL]`
- `seccompProfile: RuntimeDefault`
