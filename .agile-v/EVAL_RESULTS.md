# Evaluation Results — Aisphere IAM

> Eval flywheel for Gate 2 readiness. `eval_gate_status` must be PASS or WAIVED with approver ref for release.

## Cycle C1 — Requirements Recovery

| Dimension | Result | Evidence | Notes |
|-----------|--------|----------|-------|
| System Understanding | PASS | understanding/system_overview.md, understanding/understanding_gate_decision.md | Confidence: Medium |
| Requirements Recovery | PASS | requirements/requirements.md (63 REQs) | Approved with P0/P1/P2 priorities |
| Traceability Matrix | PASS | traceability/implementation_traceability_matrix.md | Unit evidence only |
| Gap Analysis | PASS | traceability/traceability_gaps.md (17 gaps) | P0 gaps identified |
| Architecture Convergence | PASS | PR #40 merged; legacy Organization removed | GAP-IAM-001 closed |
| Integration Tests | PASS | 14/14 integration checks passed against aisphere-dev | GAP-IAM-005~009 partially closed |
| Audit Observability | PASS | Audit writes to PostgreSQL iam_audit_logs table via auditx.NewPostgresStore | ✅ GAP-IAM-010 closed |
| Grant Expiry | PASS | Expiry executor implemented (ExpireDueGrants + Dapr Job) + unit tests | ✅ GRANT-006 closed |
| Performance/Reliability | NOT_EVALUATED | No SLOs, no load tests | GAP-IAM-016 |

**eval_gate_status:** IMPROVED
**eval_run_id:** C1-002

## Remaining for Gate 2 PASS

1. Add identity mode matrix test (DIR-007) — P0
2. Add subject lookup test (AUTHZ-RT-007) — P0
3. Add projection durability tests (PROJ-004~007) — P1
4. Add fault injection tests (ENG-004) — P2
5. Add error matrix contract tests (ENG-005) — P2

## Permission Semantic Tests (新增)

| 场景 | 测试内容 | 状态 |
|------|---------|------|
| A1 | Zone 权限授予与撤销（grant → allow → revoke → deny） | ✅ 通过 |
| A2 | Group member 继承 view 权限 | ✅ 通过 |
| A3 | Project viewer 继承 read 权限 | ✅ 通过 |
| A4 | Grant 授权与撤销（owner → edit） | ✅ 通过 |
| B1 | 用户加入组获得 view 权限 | ✅ 通过 |
| B2 | 用户移出组失去 view 权限 | ✅ 通过 |
| B3 | 子组通过 parent 继承父组 view | ✅ 通过 |
| C1 | Project viewer 继承到 SkillSpace view | ✅ 通过 |
| C2 | SkillSpace editor 继承到 Skill edit | ✅ 通过 |
| C3 | Zone member 继承到 Project read | ✅ 通过 |
| D1 | 无权限用户被拒绝 | ✅ 通过 |
| D2 | 错误权限（member 不能 manage_users） | ✅ 通过 |
| E1 | Group 权限生命周期（无权限→赋权→创建→撤销→不能创建） | ✅ 通过 |
| E2 | Grant 授权生命周期（写 owner→删 owner） | ✅ 通过 |
| E3 | 成员管理权限生命周期（无权限→赋权→分配→撤销→不能分配） | ✅ 通过 |

## Gateway E2E Tests (新增)

| 测试 | 结果 |
|------|:----:|
| IAM Health | ✅ |
| IAM Ready | ✅ |
| Get Schema | ✅ |
| Check Permission | ✅ |
| Write Relationship | ✅ |
| Verify grant | ✅ |
| Delete Relationship | ✅ |
| Deny after revoke | ✅ |
| List users | ✅ |
| List groups | ✅ |
| List role templates | ✅ |
| List capabilities | ✅ |
| No-auth denied | ✅ |

## Integration Environment

- **Environment:** aisphere-dev (36.137.200.194) K8s cluster
- **Available services:** Casdoor, SpiceDB, PostgreSQL, DTM, IAM (already deployed)
- **Integration tests:** 14/14 passed (service health, DB, Casdoor, SpiceDB, DTM, resource defaults)