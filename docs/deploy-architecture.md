# IAM 前后端部署架构

## 一、整体架构

```text
用户浏览器
  ↓ HTTPS :30723
Envoy Gateway (envoy-gateway-system)
  ↓
HTTPRoute: iam-console (aisphere)
  ├── /          → aisphere-iam-frontend:3000  (前端 SPA)
  └── /v1/iam/*  → aisphere-iam:18080          (后端 API)
```

## 二、组件清单

### 2.1 IAM 后端 (aisphere-iam)

| 项目 | 值 |
|------|-----|
| **镜像** | `registry.cn-beijing.aliyuncs.com/ainfracn/aisphere-iam:sha-03b3241` |
| **Deployment** | `aisphere-iam` (namespace: `aisphere`) |
| **Service** | `aisphere-iam` (ClusterIP, 18080 HTTP / 19080 gRPC) |
| **ConfigMap** | `aisphere-iam-config` → `/app/configs/config.yaml` |
| **Secret** | `aisphere-iam-secrets` (环境变量注入) |
| **副本数** | 1 |
| **认证模式** | `gateway_trusted`（信任 Envoy Gateway 注入的 headers） |
| **internal_call** | `enabled: false`（安全边界由 NetworkPolicy 保证） |

**环境变量（来自 Secret）：**

| 变量 | 来源 |
|------|------|
| `POSTGRES_DSN` | `aisphere-iam-secrets.postgres-dsn` |
| `CASDOOR_CLIENT_ID` | `aisphere-iam-secrets.casdoor-client-id` |
| `CASDOOR_CLIENT_SECRET` | `aisphere-iam-secrets.casdoor-client-secret` |
| `CASDOOR_M2M_CLIENT_ID` | `aisphere-iam-secrets.casdoor-m2m-client-id` |
| `CASDOOR_M2M_CLIENT_SECRET` | `aisphere-iam-secrets.casdoor-m2m-client-secret` |
| `INTERNAL_TOKEN` | `aisphere-iam-secrets.internal-token` |
| `SPICEDB_TOKEN` | `aisphere-iam-secrets.spicedb-token` |
| `DTM_BRANCH_SECRET` | `aisphere-iam-secrets.dtm-branch-secret` |

### 2.2 IAM 前端 (aisphere-iam-frontend)

| 项目 | 值 |
|------|-----|
| **镜像** | `registry.cn-beijing.aliyuncs.com/ainfracn/aisphere-iam-frontend:latest` |
| **Deployment** | `aisphere-iam-frontend` (namespace: `aisphere`) |
| **Service** | `aisphere-iam-frontend` (ClusterIP, 3000) |
| **副本数** | 1 |
| **环境变量** | `NODE_ENV=production`, `PORT=3001` |

> 注意：前端当前没有注入 `NEXT_PUBLIC_IAM_URL` 等环境变量，使用代码中的默认值。

### 2.3 Envoy Gateway

| 项目 | 值 |
|------|-----|
| **控制器** | `envoy-gateway` (namespace: `envoy-gateway-system`) |
| **数据面** | `envoy-aisphere-aisphere-gateway-a9bdf3e3` (namespace: `envoy-gateway-system`) |
| **Service 类型** | LoadBalancer（无外部 LB 时退化为 NodePort） |
| **HTTP NodePort** | `30936` |
| **HTTPS NodePort** | `30723` |
| **GatewayClass** | `aisphere-gateway-class` |
| **Gateway** | `aisphere-gateway` (namespace: `aisphere`) |

## 三、Gateway 配置

### 3.1 GatewayClass

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: aisphere-gateway-class
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
```

### 3.2 Gateway

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: aisphere-gateway
  namespace: aisphere
spec:
  gatewayClassName: aisphere-gateway-class
  listeners:
    - name: http
      port: 80
      protocol: HTTP
      allowedRoutes:
        namespaces:
          from: Same
    - name: https
      port: 443
      protocol: HTTPS
      hostname: "*.weagent.cc"
      tls:
        certificateRefs:
          - name: weagent-cc-tls
      allowedRoutes:
        namespaces:
          from: Same
```

> **关键约束**：`allowedRoutes.namespaces.from: Same` 表示只接受同 namespace（`aisphere`）的 HTTPRoute。

### 3.3 聚合 HTTPRoute: iam-console

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: iam-console
  namespace: aisphere
spec:
  hostnames:
    - iam.weagent.cc
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: aisphere-gateway
      namespace: aisphere
      sectionName: https
  rules:
    # 前端页面
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: aisphere-iam-frontend
          port: 3000
    # IAM 后端 API
    - matches:
        - path:
            type: PathPrefix
            value: /v1/iam
      backendRefs:
        - name: aisphere-iam
          port: 18080
```

### 3.4 其他 HTTPRoute

| 名称 | Hostname | 后端 |
|------|----------|------|
| `hub-http` | `api.weagent.cc` | `aisphere-hub:18001` |
| `casdoor-public-http` | `casdoor.weagent.cc` | `casdoor:8000` |
| `gitlab-http` | `gitlab.weagent.cc` | `gitlab:80` |

## 四、请求链路

### 4.1 用户访问 IAM 前端

```text
浏览器 → https://iam.weagent.cc:30723/
  ↓ DNS 解析到 36.137.200.194
  ↓ Envoy Gateway 匹配 hostname=iam.weagent.cc
  ↓ 匹配 HTTPRoute iam-console 的 / 规则
  ↓ 转发到 aisphere-iam-frontend:3000
  ↓ 返回前端 SPA 页面
```

### 4.2 前端调用 IAM API

```text
前端 SPA → GET /v1/iam/me
  ↓ Envoy Gateway 匹配 hostname=iam.weagent.cc
  ↓ 匹配 HTTPRoute iam-console 的 /v1/iam 规则
  ↓ 转发到 aisphere-iam:18080
  ↓ IAM 后端 Kernel middleware 从 x-aisphere-* headers 恢复 Principal
  ↓ 返回用户信息
```

### 4.3 OIDC 登录流程

```text
用户访问 https://iam.weagent.cc/
  ↓ Envoy 检查 OIDC session cookie
  ↓ 无 session → 302 重定向到 Casdoor
  ↓ 用户登录 Casdoor
  ↓ Casdoor 回调到 Envoy (/oauth2/callback)
  ↓ Envoy 完成 code exchange，设置 session cookie
  ↓ 重定向回原始请求 URL
  ↓ 后续请求携带 session cookie
  ↓ Envoy 注入 x-aisphere-external-* headers
  ↓ 后端收到可信身份
```

## 五、安全边界

### 5.1 认证

- **外部认证**：Envoy Gateway OIDC SecurityPolicy（当前示例中已定义但未部署到集群）
- **后端认证**：`gateway_trusted` 模式，信任 Envoy 注入的 header
- **internal_call**：已关闭（`enabled: false`），安全由 NetworkPolicy 保证

### 5.2 网络安全

- **Service 类型**：所有后端服务均为 `ClusterIP`，不对外暴露
- **NetworkPolicy**：`deploy/networkpolicy.yaml` 限制只允许 Envoy Gateway 访问 IAM 端口
- **外部入口**：仅 Envoy Gateway 的 NodePort（30723/30936）对外暴露

### 5.3 敏感信息

- **ConfigMap**：只存非敏感配置，敏感值使用 `${VAR}` 环境变量引用
- **Secret**：`aisphere-iam-secrets` 包含所有敏感信息（数据库 DSN、Casdoor 凭据、SpiceDB token 等）
- **部署前置检查**：`make deploy-apply` 会检查 Secret 是否存在，不存在时中止

## 六、部署流程

### 6.1 构建与推送

```bash
# 推送代码触发 GitHub Actions 自动构建
git push origin main

# 或手动触发
gh workflow run docker-acr.yml --ref main
```

GitHub Actions 自动：
1. 构建 Docker 镜像
2. 推送到阿里云 ACR（`registry.cn-beijing.aliyuncs.com/ainfracn/aisphere-iam`）
3. Tag 格式：`sha-<git-commit-hash>`

### 6.2 部署到 K8s

```bash
# 1. 确保 Secret 已创建（仅首次）
kubectl create secret generic aisphere-iam-secrets -n aisphere \
  --from-literal=postgres-dsn="postgres://..." \
  --from-literal=casdoor-client-id="..." \
  --from-literal=casdoor-client-secret="..." \
  --from-literal=casdoor-m2m-client-id="..." \
  --from-literal=casdoor-m2m-client-secret="..." \
  --from-literal=internal-token="..." \
  --from-literal=spicedb-token="..." \
  --from-literal=dtm-branch-secret="..."

# 2. 更新镜像并应用配置
kubectl -n aisphere set image deployment/aisphere-iam \
  iam=registry.cn-beijing.aliyuncs.com/ainfracn/aisphere-iam:sha-<commit>

# 3. 或使用 make 命令（含 Secret 前置检查）
make deploy-apply
```

### 6.3 验证

```bash
# 健康检查
curl -k https://36.137.200.194:30723/healthz

# 通过 Host header 测试 IAM API
curl -k -H "Host: iam.weagent.cc" https://36.137.200.194:30723/v1/iam/me

# 查看 Pod 状态
kubectl -n aisphere get pods -l app=aisphere-iam

# 查看日志
kubectl -n aisphere logs deployment/aisphere-iam --tail=50
```

## 七、已知问题

| 问题 | 影响 | 状态 |
|------|------|------|
| OIDC SecurityPolicy 未部署 | 前端和 API 路由没有 OIDC 保护，`/v1/iam/me` 收不到 `x-aisphere-external-*` headers | ⚠️ 示例已提供，需手动部署 |
| `publish: DISABLED` 生成器问题 | Kernel v0.4.1 generator 未正确处理 `gateway.publish: DISABLED` | ⚠️ 已临时删除 ExternalAuthorize |
| 前端无 `NEXT_PUBLIC_IAM_URL` | 前端使用默认值，跨域场景需配置 | ⚠️ 需在 deployment 中注入 |
| 前端 `ignoreBuildErrors: true` | `next.config.ts` 中设置了忽略 TS 错误 | ⚠️ 建议删除 |
| 单副本部署 | 所有服务均为 1 副本，无 HA | ⚠️ 生产需调整 |
| 无 HPA | 没有配置水平自动扩缩容 | ⚠️ 生产需补充 |