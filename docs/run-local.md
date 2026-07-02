# IAM 本地运行

推荐目录结构：

```text
E:\coding\aisphereio\
  kernel\
  aisphere-iam\
  aisphere-gateway\
```

如果 Kernel generator 有本地改动，先安装本地工具链：

```powershell
cd E:\coding\aisphereio\aisphere-iam
make tools-local KERNEL_LOCAL=..\kernel
make api
make proto-check
make test
```

如果只使用已发布 Kernel 版本：

```powershell
make tools
make api
make proto-check
make test
```

启动前确认外部依赖：

- Casdoor：`security.authn.enabled=true` 时必须可访问。
- SpiceDB：`security.authz.enabled=true` 且 `dev_allow_all=false` 时必须可访问。
- etcd：`gateway.route_registry.provider=etcd` 时必须可访问，用于向 Gateway 注册 route manifest。

默认端口：

```text
IAM HTTP:    0.0.0.0:18080
IAM gRPC:    0.0.0.0:19080
IAM metrics: 127.0.0.1:19180
```

本地启动：

```powershell
$env:CASDOOR_CLIENT_SECRET="<secret>"
$env:SPICEDB_TOKEN="<token>"
make run
```

登录 URL smoke test：

```powershell
curl "http://127.0.0.1:18080/v1/iam/login-url?redirect_uri=http://localhost:3000/callback&state=aisphere-hub&scope=read"
```

生产部署时，IAM direct HTTP 端口应仅在内网开放；公网入口统一走 Gateway。
