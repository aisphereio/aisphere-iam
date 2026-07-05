# IAM / Kernel AuthN-AuthZ / Gateway AccessX 定稿设计

## 1. 总体结论

Aisphere 统一采用以下边界：

```text
Casdoor = 唯一登录系统 + 唯一 OAuth/OIDC Token Issuer + User/Group 主数据
Gateway = 外部认证边界，本地校验 Casdoor JWT，注入可信 Principal
IAM     = 身份/权限控制面：用户组代理管理、资源/Grant、审计、AuthzService
SpiceDB = 唯一业务授权计算图
Kernel  = provider-neutral authn/authz/accessx/auditx/gatewayx 接口
```

核心约束：

1. **平台不再自签 IAM JWT。**
2. **Casdoor `access_token` / `id_token` 就是平台合法 JWT。**
3. **Gateway 使用 Casdoor OIDC Discovery + JWKS 本地校验 JWT。**
4. **IAM 不参与每个请求的 token 校验，不做全站 authn 瓶颈。**
5. **内部微服务默认 `gateway_trusted`，信任 Gateway 注入的 `X-Aisphere-*` headers。**
6. **业务权限仍然走 AccessX + IAM AuthzService + SpiceDB，不直接相信 JWT groups/roles。**
7. **IAM 使用 Casdoor M2M service application 调 Casdoor user/group 管理 API。**

---

## 2. JWKS 安全理解

JWKS 是 Casdoor 公开发布的 **公钥集合**，不是私钥。

```text
Casdoor private key  -> 签发 JWT
Casdoor JWKS public key -> 验证 JWT 签名
```

公钥公开不是泄露，因为公钥只能验证签名，不能签发 token，不能伪造 admin token，不能反推出私钥。

真正风险不是“JWKS 公开”，而是：

1. Gateway 信了错误的 discovery/JWKS 地址。
2. 只验签，不校验 `iss`。
3. 不校验 `aud`。
4. 不校验 `exp/nbf/iat`。
5. 不限制 `alg`。
6. 不清理客户端伪造的 `X-Aisphere-*` headers。

因此 Gateway 必须固定配置：

```yaml
security:
  authn:
    provider: casdoor
    casdoor:
      issuer: http://36.137.200.194:30082
      discovery_url: http://36.137.200.194:30082/.well-known/openid-configuration
      jwks_url: http://36.137.200.194:30082/.well-known/jwks
      audience: [bbdcfc272e2b990cb923]
      allowed_owners: [aisphere]
      allowed_algs: [RS256, RS512, ES256, ES512]
```

---

## 3. 最终请求链路

```text
Browser / CLI
  ↓ Authorization: Bearer <casdoor_access_token>
Gateway gwauthn
  ↓ 拉取/缓存 Casdoor JWKS
  ↓ 验签 + iss/aud/exp/owner/alg 校验
  ↓ claims -> Kernel authn.Principal
  ↓ 删除客户端原始 X-Aisphere-* headers
  ↓ 注入 Gateway 可信 X-Aisphere-* headers
Gateway accessx
  ↓ 调 IAM AuthzService / SpiceDB Check
Internal Service
  ↓ gateway_trusted 模式读取 Principal
  ↓ 执行业务逻辑
```

Gateway 认证通过后注入：

```http
X-Aisphere-Auth-Verified: true
X-Aisphere-Subject: <stable-user-id>
X-Aisphere-Subject-Type: user
X-Aisphere-Provider: casdoor
X-Aisphere-External-ID: aisphere/alice
X-Aisphere-Owner: aisphere
X-Aisphere-Username: alice
X-Aisphere-Email: alice@example.com
X-Aisphere-Groups: engineering-platform,sig-agent-runtime
```

Gateway 注入前必须清理所有入站 `X-Aisphere-*`，防止 header spoofing。

---

## 4. IAM 的定位

IAM 不再签发平台 JWT。IAM 负责：

- `BuildLoginURL`：返回 Casdoor hosted login URL。
- `ExchangeCode`：标准 OAuth code -> Casdoor token exchange，不二次签发 token。
- `RefreshToken`：代理 Casdoor refresh token flow。
- `BuildLogoutURL`：返回 Casdoor logout URL。
- `GetMe`：返回 Gateway 已认证 Principal，可选用原始 bearer token 补全 Casdoor profile。
- User/Group 管理：IAM 先查 SpiceDB 权限，再用 Casdoor M2M 修改 Casdoor。
- Resource/Grant 控制面：IAM DB 是 source of truth，SpiceDB 是授权投影。
- AuthzService：Gateway 或服务调用 IAM AuthzService 做业务授权。

---

## 5. IAM 入站认证模式

IAM 通常部署在 Gateway 后面，因此默认：

```yaml
security:
  authn:
    enabled: true
    mode: gateway_trusted
    provider: casdoor
```

含义：

```text
登录/refresh/logout/token exchange：IAM 作为 OAuth client 调 Casdoor。
普通已认证业务请求：IAM 读取 Gateway 注入的 Principal，不重复验 Casdoor JWT。
```

如果 IAM 被单独暴露给外部，必须改为：

```yaml
security:
  authn:
    mode: casdoor_jwt
```

并启用本地 Casdoor JWT verification。

---

## 6. Casdoor M2M

IAM 后端创建专用 Casdoor Application：

```text
Organization: aisphere
Application: iam-service
Grant type: client_credentials
```

用途：

```text
IAM 调 Casdoor AddUser/UpdateUser/AddGroup/UpdateGroup/AddGroupMember
```

链路：

```text
alice 请求 IAM 创建 group
  ↓
Gateway 已认证 alice
  ↓
IAM Check SpiceDB: platform:aisphere#manage_groups@user:alice
  ↓ allowed
IAM 用 iam-service client_credentials 调 Casdoor 管理 API
  ↓
IAM 同步 group/member relationship 到 SpiceDB
  ↓
IAM 写审计 actor=alice, technical_actor=iam-service
```

禁止：

- 长期使用 `built-in/admin`。
- 前端直接调 Casdoor 管理 API。
- 使用真实用户 token 直接触发 Casdoor 管理变更。

---

## 7. JWT groups/roles 的使用边界

Casdoor token 里的 groups/roles 只用于：

- 前端展示。
- 日志上下文。
- 审计上下文。
- 同步提示。

不得用于最终业务放行：

```text
不要：if group in token.groups { allow write }
应该：Check(user, resource, permission) -> SpiceDB
```

这样用户从 group 移除后，只要 IAM 同步删除 SpiceDB relationship，下一次权限 Check 立即生效，即使旧 JWT 里仍带旧 group。

---

## 8. Logout / token 失效

第一版采用 JWT 本地验签：

```text
Gateway 本地校验 JWT 签名和 exp
logout 后旧 access token 可能在 exp 前仍可本地验签通过
```

推荐策略：

1. access token TTL 设置为 15~30 分钟。
2. 高危接口在本地验签后额外调用 IAM/Casdoor introspection。
3. 后续再接入 logout event / token denylist。

---

## 9. 最终定稿

```text
AuthN：Gateway 本地使用 Casdoor JWKS 验证 Casdoor JWT。
AuthZ：Gateway / 服务通过 IAM AuthzService 调 SpiceDB Check。
Token：Casdoor 是唯一 token issuer，Aisphere 不自签平台 JWT。
Session：登录、refresh、logout、会话保持由 Casdoor 负责。
Internal：服务默认 gateway_trusted，只消费 Gateway 注入的 Principal。
Control Plane：IAM 负责用户组代理管理、资源/Grant、审计和 SpiceDB 投影。
```
