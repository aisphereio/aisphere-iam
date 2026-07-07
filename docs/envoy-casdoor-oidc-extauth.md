# Envoy Gateway + Casdoor OIDC + IAM ExtAuth 接入

本文定义 Aisphere IAM 在第一阶段 Authn 体系中的职责。

## 1. 阶段定位

第一阶段不要求 IAM 自己成为 OIDC Provider。

```text
Casdoor = OIDC Provider
Envoy Gateway = OIDC/JWT 验证、claimToHeaders、ExtAuth 调用
Aisphere IAM = external identity -> principal 映射、资源级授权、internal JWT 签发
```

IAM 不再要求 Hub/Runtime/Git 等业务服务直接理解 Casdoor token。业务服务只读取 Gateway/IAM 注入的 Aisphere principal。

## 2. 请求链路

```text
Browser / CLI / Agent
  -> Envoy Gateway
  -> Gateway 清理客户端伪造的 x-aisphere-* / x-internal-*
  -> Gateway OIDC/JWT 验证 Casdoor token
  -> Gateway claimToHeaders 提取 x-aisphere-external-*
  -> Gateway 调 IAM /v1/extauth/check
  -> IAM 映射 principal + 执行 authz
  -> IAM 返回 x-aisphere-principal / x-aisphere-internal-jwt
  -> Gateway 注入 header 后转发到上游业务服务
```

## 3. IAM ExtAuth Endpoint

标准路径：

```text
POST /v1/extauth/check
```

兼容路径：

```text
POST /internal/iam/ext-authz
```

推荐新集成统一使用 `/v1/extauth/check`。旧 Gateway 配置可在迁移期继续使用 `/internal/iam/ext-authz`。

## 4. Gateway 传给 IAM 的 Header

ExtAuth 请求必须至少包含：

```text
Authorization
Cookie
X-Request-Id
X-Forwarded-For
X-Forwarded-Proto
X-Forwarded-Host
x-aisphere-external-sub
x-aisphere-external-email
```

其中：

| Header | 来源 | 用途 |
|---|---|---|
| `Authorization` | Client / Gateway | Casdoor access token / API token |
| `Cookie` | Browser | OIDC session 场景可用 |
| `x-aisphere-external-sub` | Gateway claimToHeaders | Casdoor subject |
| `x-aisphere-external-email` | Gateway claimToHeaders | 外部邮箱 |
| `X-Forwarded-*` | Gateway | 审计和上下文 |

IAM 必须把 `x-aisphere-external-*` 视为 Gateway 提供的“外部身份线索”，不能直接当成内部 principal。

## 5. IAM 处理流程

```text
1. 读取 Authorization / Cookie / x-aisphere-external-*。
2. 验证 Casdoor JWT，或信任 Gateway 已校验并进行最小一致性检查。
3. 通过 external issuer + external subject 映射 Aisphere user。
4. 解析 org/project context。
5. 根据 route metadata/path/method 推导 resource/action。
6. 调用权限引擎检查是否允许。
7. 写入 audit decision。
8. 返回 Aisphere principal headers。
9. 可选签发短期 x-aisphere-internal-jwt。
```

## 6. 成功响应 Header

IAM ExtAuth 成功时返回 `200 OK`，并带回：

```http
x-aisphere-principal: user:u_123
x-aisphere-user-id: u_123
x-aisphere-org-id: org_001
x-aisphere-project-id: proj_001
x-aisphere-roles: owner,editor
x-aisphere-authz-decision-id: dec_01HX...
x-aisphere-internal-jwt: eyJhbGciOiJSUzI1NiIs...
```

Header 语义：

| Header | 说明 |
|---|---|
| `x-aisphere-principal` | 内部主体，格式如 `user:u_123` / `service:hub` |
| `x-aisphere-user-id` | Aisphere user id，不是 Casdoor sub |
| `x-aisphere-org-id` | 当前组织上下文 |
| `x-aisphere-project-id` | 当前项目上下文 |
| `x-aisphere-roles` | 当前上下文下的角色摘要，仅供快速判断/展示 |
| `x-aisphere-authz-decision-id` | IAM 审计决策 ID |
| `x-aisphere-internal-jwt` | IAM 签发的短期内部身份 JWT |

## 7. 失败语义

| 场景 | HTTP Status | 说明 |
|---|---:|---|
| 未认证 | 401 | 无 token、token 无效、session 不可信 |
| 无权限 | 403 | 身份有效但资源权限不足 |
| IAM 异常 | 500 | Gateway 应 fail closed |

Gateway 必须设置：

```yaml
failOpen: false
```

## 8. Internal JWT

### 8.1 用途

`x-aisphere-internal-jwt` 用于让后端二次确认：

```text
该 principal 是 IAM 签发的
该请求确实经过 Gateway/IAM auth path
principal/header 未在内部被伪造
```

### 8.2 Claim 示例

```json
{
  "iss": "https://iam.aisphere.local",
  "aud": "aisphere-internal-services",
  "sub": "user:u_123",
  "typ": "internal_gateway_principal",
  "external_iss": "https://casdoor.aisphere.local",
  "external_sub": "casdoor_user_sub",
  "org_id": "org_001",
  "project_id": "proj_001",
  "roles": ["owner", "editor"],
  "decision_id": "dec_01HX...",
  "source": "envoy-gateway",
  "iat": 1710000000,
  "exp": 1710000300
}
```

### 8.3 规则

```text
1. internal JWT 只由 IAM 签发。
2. 有效期建议 1~5 分钟。
3. 后端通过 IAM JWKS 验签。
4. 后端校验 internal JWT sub 与 x-aisphere-principal 一致。
5. Gateway 必须先清理客户端传入的 x-aisphere-internal-jwt。
6. 禁止 x-jwt-secret / x-auth-secret 模式。
```

## 9. Casdoor External Identity 映射

映射主键：

```text
external_issuer + external_subject
```

不要使用 email 作为唯一映射主键。email 可变且可能在不同 IdP 中重复。

推荐表意：

```text
identity_provider = casdoor
external_issuer   = https://casdoor.aisphere.local
external_subject  = <token.sub>
aisphere_user_id  = u_xxx
```

## 10. Route Metadata 到资源动作

IAM ExtAuth 优先使用 Gateway route metadata：

```text
aisphere.io/service
aisphere.io/auth-mode
aisphere.io/resource-type
aisphere.io/action
```

没有 metadata 时，可以降级从 method/path 推导，但必须记录 audit warning，避免长期依赖 path 解析。

## 11. 与旧 IAMAuthService 的边界

| 能力 | 第一阶段归属 |
|---|---|
| 浏览器登录跳转 | Envoy Gateway OIDC + Casdoor |
| Casdoor token 验证 | Gateway JWT；IAM 可二次校验 |
| `GetMe` | Hub/IAM 读取 principal 或调用 IAM Directory |
| 资源级 CheckPermission | IAM |
| ExtAuth | IAM |
| internal JWT | IAM |
| service token | IAM |

旧的 `BuildLoginURL` / `ExchangeCode` 仍可保留给非 Gateway 场景或兼容模式，但普通业务入口默认不再由业务服务自己处理浏览器 OIDC 登录。

## 12. 验收

```bash
# public route 不应触发 IAM ExtAuth
curl -i https://hub.aisphere.local/healthz

# protected route 没 token 应 302 或 401
curl -i https://hub.aisphere.local/api/v1/agents

# 带合法 Casdoor token 应进入 ExtAuth 并返回 principal
curl -i https://hub.aisphere.local/api/v1/agents \
  -H "Authorization: Bearer <casdoor-token>"

# 伪造 principal 必须被 Gateway 清理
curl -i https://hub.aisphere.local/api/v1/agents \
  -H "x-aisphere-principal: user:admin"
```
