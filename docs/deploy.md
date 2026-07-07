# IAM 部署方案

IAM 运行时只负责服务本身。Gateway API 路由清单由 proto 中的 `google.api.http` 和 `aisphere.access.v1.policy` 生成，`config.yaml` 不再维护公开/认证/内部路由列表，也不再向 etcd 写运行时路由事实。

最新入口安全体系见 [`docs/envoy-casdoor-oidc-extauth.md`](envoy-casdoor-oidc-extauth.md)。

## 事实源

| 内容 | 事实源 |
|---|---|
| HTTP/gRPC API | `api/**.proto` |
| 路由暴露等级 | `aisphere.access.v1.policy.exposure` |
| Gateway API YAML | `make deploy` 生成到 `deploy/generated` |
| 服务 Deployment/Service/ConfigMap | `deploy/*.yaml` |
| 公开/认证/内部路由列表 | 生成产物，不写入 `configs/config*.yaml` |

## Gateway API + Casdoor OIDC + IAM ExtAuth

第一阶段入口链路：

```text
Client
  -> Envoy Gateway
  -> Gateway 清理客户端伪造的 x-aisphere-* / x-internal-*
  -> Gateway OIDC/JWT 验证 Casdoor token
  -> Gateway claimToHeaders 写入 x-aisphere-external-*
  -> Gateway 调用 IAM /v1/extauth/check
  -> IAM external identity -> Aisphere principal
  -> IAM 返回 Gateway-controlled x-aisphere-* headers 和 x-aisphere-internal-jwt
  -> Envoy Gateway 转发到 upstream service
  -> upstream service 使用 gateway_trusted authn 恢复 Principal
```

推荐 ExtAuth 端点：

```text
POST /v1/extauth/check
```

兼容旧路径：

```text
POST /internal/iam/ext-authz
```

`/v1/extauth/check` 语义：

- 输入使用请求头 `Authorization: Bearer <casdoor-token>`，或 Gateway OIDC session 上下文。
- Gateway 可先通过 `claimToHeaders` 提供 `x-aisphere-external-sub`、`x-aisphere-external-email`。
- IAM 不直接信任 Casdoor `sub` 为内部用户 ID，而是执行 external identity mapping。
- 成功时返回 `200`，并在 response headers 写入 `x-aisphere-principal`、`x-aisphere-user-id`、`x-aisphere-org-id`、`x-aisphere-project-id` 等。
- 成功时可同时写入 `x-aisphere-internal-jwt`，后端服务在 `gateway_trusted` 模式下用 IAM JWKS 二次校验。
- 失败时返回 `401` 或 `403`，Gateway 不应继续转发。

Envoy/Gateway 必须先清理外部伪造头：

```text
x-aisphere-external-sub
x-aisphere-external-email
x-aisphere-external-name
x-aisphere-external-username
x-aisphere-principal
x-aisphere-user-id
x-aisphere-org-id
x-aisphere-project-id
x-aisphere-roles
x-aisphere-authz-decision-id
x-aisphere-internal-jwt
x-internal-jwt
```

后端服务默认仍使用 gateway-trusted 心智：

```yaml
security:
  authn:
    enabled: true
    mode: gateway_trusted
  internal_call:
    enabled: true
    header: x-aisphere-internal-jwt
    jwks_uri: https://iam.aisphere.local/.well-known/jwks.json
```

`oidc_jwt/casdoor_jwt` 可用于高安全服务的二次校验模式，但不是普通业务服务默认心智负担。普通业务服务默认读取 IAM 返回的 Aisphere principal，并可校验 internal JWT。

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
- 集群已安装 Envoy Gateway CRD，包括 `SecurityPolicy`、`ClientTrafficPolicy` 等。
- `aisphere-system` namespace 下存在生成器配置中引用的 Gateway。
- Casdoor 中存在 Gateway OIDC client：`aisphere-gateway`。
- Gateway OIDC redirect URL 已加入 Casdoor application allowed redirect URIs。
- Gateway OIDC/JWT issuer 与 Casdoor token `iss` 完全一致。
- Gateway ExternalAuth 指向 IAM 的 `http://aisphere-iam.aisphere:18080/v1/extauth/check`。
- Gateway 转发到上游服务时只保留或注入 IAM 返回的 `x-aisphere-*` 和 `x-aisphere-internal-jwt` 可信头。
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

这些路由信息由 proto contract 和 Kernel v0.2.5+ 的生成器产出，部署时应用 `deploy/generated` 即可。

## 安全注意

仓库内的部署 ConfigMap 使用 `CHANGE_ME_*` 占位符。生产环境应通过平台 Secret 管理或环境专用 overlay 注入真实凭据，不应把真实数据库密码、Casdoor client secret、SpiceDB token 提交到仓库。

Deployment 已移除旧的 privileged sysctl initContainer，并默认启用：

- `runAsNonRoot`
- `allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true`
- `capabilities.drop: [ALL]`
- `seccompProfile: RuntimeDefault`
