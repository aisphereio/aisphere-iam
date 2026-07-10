# Envoy Gateway + Casdoor OIDC 接入

本文定义 Aisphere IAM 在 Gateway OIDC 阶段中的职责，以及 IAM 业务代码如何从 Kernel ctx 获取调用者身份。

## 1. 阶段定位

当前阶段只做 Gateway OIDC/JWT 接入，不启用 Gateway ExternalAuth。

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
  -> Gateway claimToHeaders 提取 x-aisphere-* 身份头
  -> Gateway 转发到 Hub/IAM/Runtime/Git
  -> 后端 Kernel v0.4.0+ middleware 从 Gateway claim header 恢复 authn.Principal
  -> 后端通过 authn.PrincipalFromContext(ctx) 获取调用者
  -> 后端按需调用 IAM 做 user/principal 映射和业务授权
```

## 3. Gateway 输出给后端的 Header

基础 external identity header：

```text
x-aisphere-external-sub
x-aisphere-external-issuer
x-aisphere-external-email
x-aisphere-external-name
x-aisphere-external-display-name
x-aisphere-external-phone
x-aisphere-external-owner
x-aisphere-external-id
x-aisphere-external-scope
```

可选内部投影 header：

```text
x-aisphere-principal
x-aisphere-user-id
x-aisphere-org-id
x-aisphere-project-id
x-aisphere-roles
x-aisphere-groups
x-aisphere-scopes
```

Kernel 的恢复规则：

```text
SubjectID = x-aisphere-principal > x-aisphere-user-id > x-aisphere-external-id > x-aisphere-external-sub
ExternalID = x-aisphere-external-id 或 x-aisphere-external-sub
Issuer = x-aisphere-external-issuer
Email = x-aisphere-external-email
Username = x-aisphere-external-name 或 x-aisphere-external-username
Name = x-aisphere-external-display-name > x-aisphere-external-name
Phone = x-aisphere-external-phone
OrgID/TenantID = x-aisphere-org-id > x-aisphere-external-owner
ProjectID = x-aisphere-project-id
Roles = x-aisphere-roles
Groups = x-aisphere-groups
Scopes = x-aisphere-scopes 或 x-aisphere-external-scope
```

业务 handler 不要自己解析 header，统一使用：

```go
principal, ok := authn.PrincipalFromContext(ctx)
```

## 4. IAM 业务代码使用规则

IAM 内部代码把 Kernel ctx 中的 `authn.Principal` 当作服务端权威身份来源：

```go
principal, ok := authn.PrincipalFromContext(ctx)
if !ok || !principal.IsAuthenticated() {
    return nil, authn.ErrMissingCredential("kernel principal is required")
}

userID := principal.SubjectID // 稳定用户 UUID
orgID := principal.OrgID      // Casdoor owner / Aisphere org 投影
```

控制面写操作不得信任请求体里的 `owner`、`created_by`、`actor` 来代表当前调用者。IAM 服务层应使用 ctx Principal 填充：

```text
CreateOrganization.Owner = ctx Principal
CreateProject.CreatedBy = ctx Principal
CreateProject.Owner = request owner 或 ctx Principal fallback
UpsertResource.CreatedBy = ctx Principal
UpsertResource.Owner = request owner 或 ctx Principal fallback
BindResource.CreatedBy = ctx Principal
GrantAccess.CreatedBy = ctx Principal
RevokeAccess.Actor = ctx Principal
```

这样可以保证 SpiceDB 关系和审计主体使用稳定的用户 UUID，而不是客户端传入的 name/email/displayName。

## 5. IAM 不再承担浏览器 OAuth 流程

当前架构下，浏览器登录、callback、code exchange、session cookie、access token forwarding 都归 Envoy Gateway 负责。

IAM 不再提供旧的前端 OAuth flow，相关接口已从 proto 中删除：

```text
/v1/iam/login-url       （已删除）
/v1/iam/auth/exchange   （已删除）
/v1/iam/auth/refresh    （已删除）
/v1/iam/auth/revoke     （已删除）
/v1/iam/logout-url      （已删除）
```

正确入口是访问受 Gateway 保护的接口，例如：

```text
https://iam.weagent.cc/v1/iam/me
```

未登录时由 Envoy Gateway 自动跳转到 Casdoor。登录完成后，Gateway 注入 claim header，IAM 后端只读取 Kernel context 中的 Principal。

## 6. IAM 在本阶段的职责

IAM 作为普通后端服务提供：

```text
1. Casdoor 用户/组织/组目录适配。
2. external_issuer + external_subject -> Aisphere user 映射查询。
3. GetMe / directory API。
4. Project / Organization / Grant / Permission API。
5. 后端服务主动调用的 CheckPermission。
```

IAM 不承担：

```text
1. Gateway ExternalAuth。
2. Gateway 请求前置授权决策。
3. 浏览器 OAuth code exchange。
4. 前端 token refresh / revoke。
5. 前端本地 token session 管理。
```

## 7. 后端映射逻辑

业务服务收到：

```http
x-aisphere-external-sub: casdoor_user_sub
x-aisphere-external-issuer: https://casdoor.weagent.cc:30723
x-aisphere-external-email: user@example.com
```

后端通过 Kernel middleware 得到 Principal 后，可以通过 IAM client 查询：

```text
external_issuer = https://casdoor.weagent.cc:30723
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

## 8. Casdoor OIDC 配置要求

每个前端应用使用独立的 Casdoor Application：

```text
IAM 管理控制台:
  client_id: aisphere-iam-web
  redirect_uri: https://iam.weagent.cc/oauth2/callback
  scopes: openid profile email

Hub 前端:
  client_id: aisphere-hub-web
  redirect_uri: https://hub.weagent.cc/oauth2/callback
  scopes: openid profile email
```

> **注意**：`/oauth2/callback` 由 Envoy Gateway 的 OIDC filter 消费，不会到达 IAM 后端。不要把它放在 `/v1/iam/` 路径下，避免误导开发者认为这是 IAM 的 API。

Envoy Gateway 的 issuer 必须与 Casdoor token 的 `iss` 完全一致。

## 9. Envoy Gateway 示例

示例清单位于：

```text
deploy/examples/envoy-casdoor-oidc-security-policy.yaml
```

该示例包含：

```text
ClientTrafficPolicy header sanitize
OIDC SecurityPolicy
JWT provider
claimToHeaders
```

不包含 ExtAuth。

## 10. 验收

```bash
# public route 不应触发登录
curl -i https://iam.weagent.cc/healthz

# 未登录访问受保护接口，应触发 OIDC 302
curl -vk https://iam.weagent.cc/v1/iam/me

# 登录后访问 GetMe，应返回 Gateway header 恢复出来的 Principal
# 浏览器访问：
# https://iam.weagent.cc/v1/iam/me

# 伪造 principal 必须被 Gateway 清理
curl -i https://iam.weagent.cc/v1/iam/me \
  -H "x-aisphere-principal: user:admin"
```

## 11. 后续阶段

后续再增加：

```text
Gateway ExternalAuth -> IAM ext-authz endpoint
IAM 返回 x-aisphere-principal
IAM 签发 x-aisphere-internal-jwt
后端校验 IAM JWKS
```
