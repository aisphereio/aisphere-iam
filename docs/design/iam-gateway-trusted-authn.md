# IAM Gateway Trusted AuthN

IAM 默认位于 Gateway 后面。外部用户请求先由 Gateway 使用 Casdoor issuer/JWKS 校验 JWT，然后 Gateway 注入：

- `X-Aisphere-Auth-Verified: true`
- `X-Aisphere-Subject`
- `X-Aisphere-Username`
- `X-Aisphere-Owner`
- `X-Aisphere-Groups`
- `X-Aisphere-Internal-Token`

IAM 在 `security.authn.mode=gateway_trusted` 下不再重复校验每个请求的 Casdoor JWT，而是：

1. 校验 `X-Aisphere-Internal-Token` 是否与配置一致；
2. 解析 Gateway 注入的 Principal；
3. 把 Principal 放入 Kernel context；
4. authz 当前可以 `dev_allow_all` 短路，后续恢复 SpiceDB。

`internal-service-token` 只证明请求来自可信 Gateway，不代表用户身份。用户身份仍来自 Casdoor JWT 校验后的 Principal。

直连测试或高安全接口可以把 `security.authn.mode` 切回 `casdoor_jwt`，复用 Kernel `authn/oidcx` 对 Casdoor JWT 做二次验签。
