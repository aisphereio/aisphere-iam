# Envoy Gateway + Casdoor OIDC 后续扩展说明

本文件原用于记录 Gateway 前置授权方案。当前版本已收敛为 OIDC-only，不启用 Gateway ExternalAuth。

当前阶段请阅读：

```text
docs/envoy-casdoor-oidc.md
```

当前阶段只实现：

```text
Casdoor OIDC
Envoy Gateway JWT 验签
claimToHeaders
header sanitize
public/authn/protected route 分组
后端 Kernel middleware 读取 x-aisphere-external-*
```

后续阶段再设计：

```text
Gateway 调 IAM /v1/extauth/check
IAM 返回 x-aisphere-principal
IAM 签发 x-aisphere-internal-jwt
Gateway headersToBackend 注入可信内部身份
```
