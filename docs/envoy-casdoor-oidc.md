# Envoy Gateway + Casdoor OIDC 接入

本文定义 Aisphere IAM 在 OIDC-only 第一阶段中的职责。

## 1. 阶段定位

第一阶段只做 Gateway OIDC/JWT 接入，不启用 Gateway ExternalAuth。

```text
Casdoor = OIDC Provider
Envoy Gateway = OIDC 登录、JWT 验签、claimToHeaders、header sanitize、路由转发
Aisphere IAM = 后端目录/权限服务；不作为 Gateway ExtAuth 服务
```

本阶段业务服务仍可以通过 IAM API 查询用户、组织、项目、角色和权限，但 Envoy Gateway 不会在转发前调用 IAM。

## 2. 请求链路

```text
Browser / CLI / Agent
  -> Envoy Gateway
  -> Gateway 清理客户端伪造的 x-aisphere-* / x-internal-*
  -> Gateway OIDC/JWT 验证 Casdoor token
  -> Gateway claimToHeaders 提取 x-aisphere-external-*
  -> Gateway 转发到 Hub/IAM/Runtime/Git
  -> 后端 Kernel middleware 读取 external identity
  -> 后端按需调用 IAM 做 user/principal 映射和业务授权
```

## 3. Gateway 输出给后端的 Header

本阶段 Gateway 只输出 external identity header：

```text
x-aisphere-external-sub
x-aisphere-external-email
x-aisphere-external-name
x-aisphere-external-username
```

语义：

| Header | 来源 | 说明 |
|---|---|---|
| `x-aisphere-external-sub` | Casdoor JWT `sub` | Casdoor subject |
| `x-aisphere-external-email` | Casdoor JWT `email` | 外部邮箱 |
| `x-aisphere-external-name` | Casdoor JWT `name` | 展示名 |
| `x-aisphere-external-username` | Casdoor JWT `preferred_username` | 外部用户名 |

这些 header 不等价于 Aisphere internal principal。

## 4. 本阶段不使用的 Header

Gateway 不输出：

```text
x-aisphere-principal
x-aisphere-user-id
x-aisphere-org-id
x-aisphere-project-id
x-aisphere-roles
x-aisphere-authz-decision-id
x-aisphere-internal-jwt
```

这些 header 预留给后续 IAM ExternalAuth 阶段。

## 5. IAM 在本阶段的职责

IAM 作为普通后端服务提供：

```text
1. Casdoor 用户/组织/组目录适配。
2. external_issuer + external_subject -> Aisphere user 映射查询。
3. GetMe / profile / directory API。
4. Project / Organization / Grant / Permission API。
5. 后端服务主动调用的 CheckPermission。
```

IAM 不承担：

```text
1. Gateway ExternalAuth。
2. Gateway 请求前置授权决策。
3. Gateway 注入 x-aisphere-principal。
4. Gateway 注入 x-aisphere-internal-jwt。
```

## 6. 后端映射逻辑

业务服务收到：

```http
x-aisphere-external-sub: casdoor_user_sub
x-aisphere-external-email: user@example.com
```

后端 Kernel middleware 或业务服务可以通过 IAM client 查询：

```text
external_issuer = https://casdoor.aisphere.local
external_subject = casdoor_user_sub
```

映射得到：

```text
aisphere_user_id
org memberships
project context
roles/grants
```

注意：不要使用 email 作为唯一映射主键。推荐使用：

```text
external_issuer + external_subject
```

## 7. Casdoor OIDC 配置要求

Casdoor application 需要配置：

```text
client_id: aisphere-gateway
redirect_uri:
  https://hub.aisphere.local/oauth2/callback
  https://iam.aisphere.local/oauth2/callback
scopes:
  openid
  profile
  email
```

Gateway 的 issuer 必须与 Casdoor token 的 `iss` 完全一致。

## 8. Envoy Gateway 示例

示例清单位于：

```text
deploy/examples/envoy-casdoor-oidc-security-policy.yaml
```

该示例只包含：

```text
ClientTrafficPolicy header sanitize
OIDC SecurityPolicy
JWT provider
claimToHeaders
```

不包含 ExtAuth。

## 9. 验收

```bash
# public route 不应触发登录
curl -i https://hub.aisphere.local/healthz

# authn/protected route 未登录应触发 OIDC 或 401
curl -i https://hub.aisphere.local/api/v1/me

# Bearer token 请求应通过 JWT 验签
curl -i https://hub.aisphere.local/api/v1/me \
  -H "Authorization: Bearer <casdoor-token>"

# 伪造 principal 必须被清理
curl -i https://hub.aisphere.local/api/v1/me \
  -H "x-aisphere-principal: user:admin"
```

## 10. 后续阶段

后续再增加：

```text
Gateway ExternalAuth -> IAM /v1/extauth/check
IAM 返回 x-aisphere-principal
IAM 签发 x-aisphere-internal-jwt
后端校验 IAM JWKS
```
