# IAM Gateway Trusted AuthN

IAM 默认位于 Envoy Gateway 后面。外部用户请求先由 Envoy Gateway 使用 Casdoor issuer/JWKS 校验 JWT，然后 Envoy Gateway 通过 `claimToHeaders` 注入可信身份 headers。

IAM 在 `security.authn.mode=gateway_trusted` 下不再重复校验每个请求的 Casdoor JWT，而是：

1. Kernel middleware 自动从 `x-aisphere-external-*` headers 恢复 `authn.Principal`；
2. 把 Principal 放入 Kernel context；
3. 业务代码通过 `authn.PrincipalFromContext(ctx)` 获取。

`gateway_trusted` 信任的不是一个 Header，而是"经过 Envoy Gateway 的受控网络路径"。生产部署必须确保：

1. IAM Service 不得通过 NodePort/LoadBalancer 暴露。
2. NetworkPolicy 只允许 Envoy Gateway Pod 访问 IAM HTTP 端口。
3. 内部服务调用不能冒充外部 Gateway Header。
4. 内部调用使用 service token、internal JWT 或 workload mTLS。