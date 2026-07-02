# IAM Kernel 合规说明

## 已接入

- `configx`：配置文件 + 环境变量加载。
- `logx`：统一结构化日志和 access log 配置。
- `metricsx`：Prometheus metrics manager。
- `dtmx`：可选分布式事务 manager。
- `authn/casdoor`：登录、token、profile、identity adapter。
- `authz/spicedb`：资源关系授权 adapter。
- `accessx`：组合 authn/authz/audit 的访问控制 guard。
- `gatewayx`：启动时注册 generated Gateway Manifest。
- `transportx/http` 与 `transportx/grpc`：HTTP/gRPC 双栈。

## 本次补齐

IAM server 现在在 HTTP 和 gRPC 入口都装配：

```text
requestinfo.Server
  -> authn.Server(allow anonymous)
  -> access.Server(accessx.Guard)
```

这意味着资源级授权不再只依赖 Gateway；即使直接访问 IAM 内网端口，`AUTHORIZED` / 带 authz rule 的接口也会经过 Kernel access chain。

## 仍需继续推进

- 当前 `IAMAuthServiceRequestInfoResolver` 是手写文件。Kernel `protoc-gen-go-kernel` 后续会生成 module-scoped fallback resolver，业务仓库不应继续扩展手写 resolver。
- Route registry 的 etcd adapter 仍在业务仓库内，后续应下沉到 Kernel runtime/contrib。
- `RefreshToken` 当前标记为 `AUTHENTICATED`，需要结合产品语义确认是否改成 PUBLIC + refresh token body 特例。
