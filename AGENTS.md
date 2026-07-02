# Aisphere IAM Agent 规范

本仓库是 Kernel 体系下的 IAM 服务仓库，不是 Kernel layout 模板仓库。AI Agent 和人类开发者必须遵守以下约束。

## 1. 模块路径

- `go.mod` 必须使用 `module github.com/aisphereio/aisphere-iam`。
- 所有内部 import 必须使用 `github.com/aisphereio/aisphere-iam/...`。
- 禁止重新引入 `module aisphere-iam` 或 `import "aisphere-iam/..."`。
- 本地多仓联调用 `go.work`，不要把短模块路径作为长期方案。

## 2. Kernel contract 优先

- 新 RPC 必须先写 proto contract。
- 对外 RPC 必须同时声明 `google.api.http` 和 `aisphere.access.v1.policy`。
- 修改 proto 后必须运行 `make api && make proto-check`。
- 如果 Kernel generator 不能表达需求，先修 Kernel generator，再改 IAM 业务代码。

## 3. 访问控制硬规则

IAM 是权限边界服务，不能只依赖 Gateway 防护。

- HTTP/gRPC server 必须接入 `requestinfo + authn + access` middleware。
- `PUBLIC` 只允许登录跳转、授权码交换等明确公开接口。
- `INTERNAL` 接口不能暴露到公网入口。
- `AUTHORIZED` 接口必须经过 `accessx.Guard`，并写 audit。
- 不允许在 service 方法里长期手写 Bearer token 解析作为主鉴权链路。

## 4. Gateway 注册

- IAM 可以在启动时把 generated Gateway Manifest 注册到 `gatewayx.RouteRegistry`。
- Route Manifest 必须来自 generated code，不允许手写外部 HTTP path 清单。
- Route registry 的 etcd adapter 后续应收敛到 Kernel runtime/contrib，不要在业务仓库复制更多实现。

## 5. 本地工具链

如果同时修改了 Kernel generator，先在本仓库安装本地 generator：

```powershell
make tools-local KERNEL_LOCAL=../kernel
make api
make proto-check
make test
```

## 6. 文档门禁

以下变化必须同步 README 或 `docs/*.md`：启动依赖、端口、Casdoor/SpiceDB/etcd 配置、access policy、Gateway route registry、Kernel generator 使用方式。
