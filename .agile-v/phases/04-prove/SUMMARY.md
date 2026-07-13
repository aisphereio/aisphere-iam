# Phase 04-Prove — Evidence Collection

> Status: IN_PROGRESS
> Date: 2026-07-13

## Completed

### Test Infrastructure
- Created test database: `aisphere_iam_test` on aisphere-dev PostgreSQL
- Created test database: `spicedb_test` on aisphere-dev PostgreSQL
- Created test config: `configs/config.test.yaml` (connects to remote services)
- Created integration test file: `internal/service/iam_integration_test.go` (build-tagged `integration`)

### Test Databases

| Database | Purpose | Connection |
|----------|---------|------------|
| `aisphere_iam_test` | IAM control-plane data | `postgres://postgres:ChangeMe_PostgreSQL_123root@36.137.200.194:30080/aisphere_iam_test` |
| `spicedb_test` | SpiceDB authorization data | Managed by SpiceDB |

### Test Configuration

Config file: `configs/config.test.yaml`

| Service | Endpoint | Auth |
|---------|----------|------|
| PostgreSQL | 36.137.200.194:30080 | postgres / ChangeMe_PostgreSQL_123root |
| Casdoor | 36.137.200.194:30082 | admin / 123 |
| SpiceDB | 36.137.200.194:30084 | preshared-key: keykeykey |

## CI/CD Pipeline

| Workflow | Trigger | Action |
|----------|---------|--------|
| `ci.yml` | PR / push | Unit tests, contract checks, build |
| `docker-aliyun.yml` | Push to main | Build Docker image, push to Aliyun registry |
| `integration-test.yml` | Docker build complete | Deploy to aisphere-dev, run integration tests |

## Integration Test Results

| Test | Result | Date |
|------|:------:|:----:|
| IAM Health Check | ✅ PASS | 2026-07-13 |
| IAM Ready Check | ✅ PASS | 2026-07-13 |
| Casdoor OIDC Configuration | ✅ PASS | 2026-07-13 |
| PostgreSQL Connection (aisphere_iam_test) | ✅ PASS | 2026-07-13 |
| SpiceDB Health | ✅ PASS | 2026-07-13 |
| IAM GetMe (auth required) | ✅ PASS (expected) | 2026-07-13 |

## Remaining Work

| Task | Priority | Status |
|------|:--------:|:------:|
| Run integration tests via GitHub Actions | P0 | ⏳ PENDING (needs TEST_SERVER_SSH_KEY secret) |
| Write end-to-end test scenarios (Project CRUD, Grant flow) | P1 | ⏳ PENDING |
| Record test results in VALIDATION_SUMMARY.md | P1 | ⏳ PENDING |