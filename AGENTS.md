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

## 7. Agile V 追溯链规范

本仓库使用 Agile V 框架管理需求→接口→实现→测试的全链路追溯。所有 Agent 必须遵守以下规范。

### 7.1 追溯链结构

```
REQ (requirements.md) ──→ ART (BUILD_MANIFEST.md) ──→ TC (TEST_SPEC.md)
需求                     实现路径                     测试用例
```

### 7.2 变更工作流

每次提交代码必须按以下顺序操作：

```
Step 1: 写/改需求
        → 更新 .agile-v/requirements/requirements.md
        → 每个需求必须有唯一 REQ-IAM-XXXX-NNN 编号

Step 2: 写/改代码
        → 在 BUILD_MANIFEST.md 中添加 ART 条目
        → 格式: | ART-NNNN | REQ-IAM-XXXX-NNN | 代码路径 | 说明 |
        → 如果 REQ 是简写（如 DIR-001），工具会自动补全为 REQ-IAM-DIR-001

Step 3: 写/改测试
        → 在 TEST_SPEC.md 中添加 TC 条目
        → 格式: | TC-NNNN | REQ-IAM-XXXX-NNN | 说明 | 类型 | 文件路径 | 状态 |
        → 状态标记: ✅ 已实现 / ❌ 缺失

Step 4: 验证追溯链
        → 运行 make traceability-check
        → 确认输出中无意外 gap
```

### 7.3 追溯链检查

```bash
# 检查追溯链完整性（不阻断）
make traceability-check

# 严格模式（有 gap 就 exit 1，用于 CI）
make traceability-check STRICT=1

# 全量验证（包含追溯链检查）
make verify
```

### 7.4 追溯覆盖率目标

| 指标 | 当前 | 目标 |
|------|------|------|
| REQ→ART 覆盖率 | 94% | 100% |
| REQ→Test 覆盖率 | 70% | 100% |
| ART 路径存在率 | 100% | 100% |

### 7.5 常见错误

- ❌ 新增代码但没有更新 BUILD_MANIFEST.md → traceability-check 会报 ART 缺失
- ❌ 新增测试但没有更新 TEST_SPEC.md → traceability-check 会报 TC 缺失
- ❌ 修改了 REQ 编号但没有同步更新 ART/TC 中的引用 → traceability-check 会报断裂
- ❌ 删除了代码文件但没有更新 BUILD_MANIFEST.md → traceability-check 会报路径不存在

### 7.6 文件位置

```
.agile-v/
├── requirements/
│   └── requirements.md        # 需求定义（REQ）
├── BUILD_MANIFEST.md          # 实现清单（ART）
├── TEST_SPEC.md               # 测试规格（TC）
├── ATM.md                     # 追溯矩阵摘要
├── CAPA_LOG.md                # 纠正预防措施
├── CHANGE_LOG.md              # 变更记录
├── DECISION_LOG.md            # 决策日志
├── EVAL_RESULTS.md            # 评估结果
├── STATE.md                   # 当前状态
└── CONTROL_MATRIX.yaml        # 控制矩阵
```
