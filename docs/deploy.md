# IAM 部署方案

IAM 运行时只负责服务本身。Gateway API 路由清单由 proto 中的 `google.api.http` 和 `aisphere.access.v1.policy` 生成，`config.yaml` 不再维护公开、认证、内部路由列表。

最新入口安全体系见 [`docs/envoy-casdoor-oidc.md`](envoy-casdoor-oidc.md)。

## 事实源

| 内容 | 事实源 |
|---|---|
| HTTP/gRPC API | `api/**.proto` |
| 路由暴露等级 | `aisphere.access.v1.policy.exposure` |
| Gateway API YAML | `make deploy` 生成到 `deploy/generated` |
| 服务 Deployment/Service/ConfigMap | `deploy/*.yaml` |

## Gateway API + Casdoor OIDC

第一阶段入口链路：

```text
Client
  -> Envoy Gateway
  -> Gateway 清理客户端伪造的 x-aisphere-* / x-internal-*
  -> Gateway OIDC/JWT 验证 Casdoor identity
  -> Gateway claimToHeaders 写入 x-aisphere-external-*
  -> Envoy Gateway 转发到 upstream service
  -> upstream service 使用 Kernel middleware 恢复 external identity
  -> upstream service 按需调用 IAM 查询用户、组织、项目和权限
```

本阶段不启用 Gateway 前置授权，不由 Gateway 注入 `x-aisphere-principal` 或 `x-aisphere-internal-jwt`。

Gateway 必须先清理外部伪造头：

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

后端服务默认读取：

```text
x-aisphere-external-sub
x-aisphere-external-email
x-aisphere-external-name
x-aisphere-external-username
```

需要内部 Aisphere user/org/project 时，由后端通过 IAM client 查询映射。

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

## 前置条件

- 集群已安装 Gateway API CRD。
- 集群已安装 Envoy Gateway CRD，包括 `SecurityPolicy`、`ClientTrafficPolicy` 等。
- `aisphere-system` namespace 下存在生成器配置中引用的 Gateway。
- Casdoor 中存在 Gateway OIDC client：`aisphere-gateway`。
- Gateway OIDC redirect URL 已加入 Casdoor application allowed redirect URIs。
- Gateway OIDC/JWT issuer 与 Casdoor identity issuer 完全一致。
- Gateway 转发到上游服务时只保留 Gateway 生成的 `x-aisphere-external-*`。
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
