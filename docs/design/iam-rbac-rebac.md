# Aisphere IAM RBAC / ReBAC 分层设计

## 1. 决策

业务权限需要 RBAC，但 RBAC 只作为产品语言和管理入口；底层统一用 ReBAC 关系图实现。

```text
RBAC = UI / RoleTemplate / GrantAccess 输入
ReBAC = SpiceDB Relationship / Permission Check / Lookup
```

不要把业务 RBAC 放到 Casdoor。Casdoor 只作为身份系统和用户目录来源。

## 2. Casdoor 边界

Casdoor 管：

- User
- Group
- Organization 身份投影
- Application / client_id
- External OIDC/SAML/OAuth Provider
- Login / Token / SSO

Casdoor 不管：

- project developer
- skill editor
- repo writer
- agent operator
- sandbox viewer

Casdoor group 可以同步或映射成 IAM group，然后作为 SpiceDB subject 使用：

```text
casdoor group: backend
  -> iam group: backend
  -> spicedb subject: group:backend#member
```

## 3. IAM RBAC 角色

### Organization

- owner
- admin
- member
- auditor

### Project

- owner
- admin
- developer
- operator
- viewer

### Skill

- owner
- editor
- reviewer
- viewer

### Git repository

- owner
- maintainer
- writer
- reader

### Agent

- owner
- editor
- operator
- executor
- viewer

### Sandbox

- owner
- operator
- viewer

## 4. 授权落地

用户操作：

```text
给 backend 组授予 project developer
```

IAM 写 grant：

```text
resource = project:p1
role_key = developer
relation = developer
subject = group:backend#member
```

SpiceDB 投影：

```text
project:p1#developer@group:backend#member
```

## 5. 为什么不能只用 RBAC

Aisphere 资源之间存在关系推导：

- project -> skill_space -> skill
- project -> git_namespace -> repository
- skill -> backing_repo
- agent -> uses_skill
- deployment -> runs_agent

例如：

```text
如果用户能 edit skill，则可以 write backing repo。
如果用户是 project developer，则默认可以编辑 project 下的 skill 和 repo。
```

这类规则用 ReBAC/SpiceDB 更自然。

## 6. 为什么不能只用 ReBAC

纯 ReBAC 对产品和管理 UI 不友好。管理员需要看到的是：

```text
项目开发者
Skill 编辑者
Repo 写入者
Sandbox 操作者
```

不是 relationship tuple、subject set 和 permission algebra。

## 7. 统一原则

- Casdoor 只做身份。
- IAM 记录 Grant 管理事实。
- SpiceDB 是业务授权计算唯一来源。
- 业务服务只调用 IAM Check / Grant / Resource API，不直接写 Casdoor Permission。
