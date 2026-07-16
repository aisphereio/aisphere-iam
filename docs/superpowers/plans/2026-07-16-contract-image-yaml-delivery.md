# IAM Contract, Image, and YAML Delivery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GitHub Actions publish immutable IAM/IAM Front images, a governed OpenAPI contract, and digest-pinned deployment YAML without any cluster mutation authority.

**Architecture:** IAM proto remains authoritative. IAM generates and validates a committed OpenAPI snapshot, IAM Front generates a Fetch client from a pinned copy, and each deployable repository renders its own Kustomize bundle after the registry returns an immutable image digest. Kernel owns the reusable transport/error contract; IAM keeps only a temporary repository-level OpenAPI validator until the next Kernel tool release is consumed.

**Tech Stack:** Go 1.26, Buf v2, grpc-gateway OpenAPI v2, Kernel generators, Orval 8, TypeScript, Fetch, TanStack Query, Vitest, Docker Buildx, Kustomize, GitHub Actions, Aliyun ACR.

---

## File map

### `aisphere-iam`

- `internal/server/generated_catalog_contract_test.go`: public test seam for generated service ownership.
- `internal/server/modules.go`, `internal/server/wiring.go`, `internal/server/http.go`, `internal/server/grpc.go`: generated module and binding convergence.
- `cmd/openapi-contract-check/main.go`: temporary IAM-side validation of the generated Swagger document.
- `cmd/openapi-contract-check/main_test.go`: independent fixtures for OpenAPI metadata, routes, and Kernel error schema.
- `Makefile`, `buf.yaml`, `.gitignore`: contract generation and compatibility commands.
- `docs/openapi/aisphere.swagger.json`: committed normalized consumer contract.
- `deploy/kustomization.yaml`: complete backend and generated Gateway route render surface.
- `.github/workflows/ci.yml`: PR verification.
- `.github/workflows/delivery.yml`: image and immutable YAML/contract bundle publication.
- `.github/workflows/docker-acr.yml`, `.github/workflows/docker-aliyun.yml`: removed duplicate publishers.
- `scripts/verify-delivery-workflow.ps1`: static workflow safety gate.

### `aisphere-iam-front`

- `openapi/aisphere.swagger.json`, `openapi/contract-lock.json`: pinned IAM contract.
- `orval.config.ts`: deterministic Fetch client generation.
- `src/lib/api/generated/`: generated backend DTO and request implementation.
- `src/lib/api/iam-fetch.ts`: Envoy cookie transport and structured Kernel errors.
- `src/lib/api/iam-fetch.test.ts`: transport seam tests.
- `src/lib/api/index.ts`, `src/hooks/use-iam.ts`: Group tracer migration.
- `scripts/verify-contract-lock.mjs`: contract digest gate.
- `package.json`, `package-lock.json`: generation, typecheck, and CI commands.
- `deploy/kustomization.yaml`: render surface whose image is replaced by digest in Actions.
- `.github/workflows/build-image.yml`: verify, publish image, and package YAML.
- `.github/workflows/deploy-k8s.yml`: removed because Actions must not mutate clusters.
- `scripts/verify-delivery-workflow.mjs`: workflow safety gate.

### `kernel`

- `cmd/openapi-contract/main.go`, `cmd/openapi-contract/main_test.go`: reusable OpenAPI normalizer/checker promoted from the IAM fixture behavior.
- `docs/contracts/openapi.md`: generated OpenAPI and Kernel HTTP error contract.
- `.github/workflows/verify.yml`: PR-HEAD verification without cross-branch file copying or automatic commits.

## Task 1: Establish isolated worktrees and clean baselines

- [ ] **Step 1: Create `agent/contract-delivery` worktrees for all three repositories**

Use each repository's ignored `.worktrees` directory. Base IAM on the branch containing the approved specification; base Kernel and IAM Front on their current HEADs.

- [ ] **Step 2: Install dependencies**

Run:

```powershell
go mod download
npm ci
```

Run `go mod download` in the Kernel and IAM worktrees. Run `npm ci` in the IAM Front worktree.

- [ ] **Step 3: Verify baseline seams**

Run:

```powershell
go test ./internal/server ./cmd/permission-manifest-check
npm test -- --run
npx tsc --noEmit --pretty false
```

Expected: exit code 0 before contract-delivery implementation begins.

## Task 2: Make generated IAM modules own every generated public service

**Files:** `internal/server/generated_catalog_contract_test.go`, `internal/server/modules.go`, `internal/server/wiring.go`, `internal/server/http.go`, `internal/server/grpc.go`

- [ ] **Step 1: Write the failing generated-catalog test**

Add a table that requires these operations to resolve through `IAMCatalog()` and the binding list:

```go
var required = []string{
    "/iam.v1.IAMDirectoryProjectionService/RetryDirectoryProjection",
    "/iam.v1.IAMAuthorizationAdminService/GetAuthorizationSchema",
}
```

Assert that the catalog contains each generated route and that the HTTP binding names include all eleven proto services.

- [ ] **Step 2: Run the targeted test and confirm red**

```powershell
go test ./internal/server -run TestGeneratedCatalogOwnsAllIAMServices -count=1
```

Expected: failure naming directory-projection and authorization-admin ownership gaps.

- [ ] **Step 3: Add both Kernel modules and service bindings**

Add `IAMDirectoryProjectionServiceKernelModule()` and `IAMAuthorizationAdminServiceKernelModule()` to `IAMModules()`. Extend `IAMBindings()` and both server constructors so `serverx.RegisterHTTPServices` / `RegisterGRPCServices` receive the matching implementations.

- [ ] **Step 4: Delete the redundant public compatibility registrations**

Remove direct authorization-admin HTTP registration and the three `/v1/iam/directory/projections:*` compatibility handlers. Keep `/internal/dtm/...` callbacks as explicit internal adapters. Move Casdoor webhook and UI-login classification into a separate test expectation so they cannot silently enter the generated business catalog.

- [ ] **Step 5: Run the targeted server tests and confirm green**

```powershell
go test ./internal/server -count=1
```

## Task 3: Govern the IAM OpenAPI artifact

**Files:** `cmd/openapi-contract-check/main.go`, `cmd/openapi-contract-check/main_test.go`, `Makefile`, `.gitignore`, `docs/openapi/aisphere.swagger.json`

- [ ] **Step 1: Write red tests for the consumer contract**

Fixture assertions must independently require:

```text
info.title = Aisphere IAM API
info.version is non-empty
every operation has operationId and at least one tag
every default error response references KernelErrorResponse
KernelErrorResponse has code, message, request_id, trace_id, metadata
no duplicate method/path pair
```

- [ ] **Step 2: Run and confirm the current Swagger fails**

```powershell
go test ./cmd/openapi-contract-check -count=1
```

Expected: red on title/version and `rpcStatus` default errors.

- [ ] **Step 3: Implement deterministic normalization and checking**

The command reads Swagger JSON, sets the IAM title/version supplied by flags, adds `KernelErrorResponse`, replaces default `rpcStatus` references, sorts output through Go's JSON encoder, validates operation identity/tag requirements, and writes the normalized file atomically.

CLI:

```text
openapi-contract-check --input docs/openapi/aisphere.swagger.json --output docs/openapi/aisphere.swagger.json --title "Aisphere IAM API" --version "dev"
```

- [ ] **Step 4: Expose Make targets**

Add:

```make
openapi-check:
	$(GO) run ./cmd/openapi-contract-check --input docs/openapi/aisphere.swagger.json --output docs/openapi/aisphere.swagger.json --title "Aisphere IAM API" --version "$(VERSION)"

breaking-check:
	$(BUF) breaking --against '.git#branch=$${GITHUB_BASE_REF:-main}'

contract-check: proto-check breaking-check api openapi-check
```

On Windows use the repository-local `.bin\buf.exe` with an explicit `BASE_REF` Make variable rather than shell-only substitution.

- [ ] **Step 5: Track and regenerate the contract**

Remove `docs/openapi/` from `.gitignore`, run `make api openapi-check`, and add the normalized JSON.

- [ ] **Step 6: Verify deterministic generation**

Run the command twice and assert `git diff --exit-code docs/openapi/aisphere.swagger.json` after the second run.

## Task 4: Add backend compatibility and workflow safety gates

**Files:** `.github/workflows/ci.yml`, `scripts/verify-delivery-workflow.ps1`

- [ ] **Step 1: Write a red static workflow test**

The PowerShell test fails unless CI contains `buf breaking`, OpenAPI validation, generated diff, Go tests, traceability, binary build, container build, and Kustomize render. It also fails if any workflow contains `kubectl apply`, `kubectl set image`, `rollout status`, kubeconfig writes, or cluster secrets.

- [ ] **Step 2: Run and confirm red**

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-delivery-workflow.ps1
```

- [ ] **Step 3: Update CI and confirm green**

CI checks out full history, installs pinned tools, runs `make contract-check deploy`, tests/builds, renders manifests, and checks the worktree. PR jobs build the Dockerfile with `push: false`.

## Task 5: Produce the backend image and delivery bundle

**Files:** `deploy/kustomization.yaml`, `.github/workflows/delivery.yml`, old Docker workflows

- [ ] **Step 1: Write a red Kustomize completeness test**

Render `deploy/` and assert it contains the IAM Deployment, Service, NetworkPolicy, and every generated authenticated/internal HTTPRoute, while containing no `Secret` document or `CHANGE_ME` value.

- [ ] **Step 2: Update the Kustomization**

Include namespace-safe application resources, network policy, and generated Gateway route directories. Keep `secret.yaml` excluded.

- [ ] **Step 3: Consolidate image publication**

Replace both Docker workflows with `delivery.yml`. It verifies first, publishes the `sha-${GITHUB_SHA::7}` tag plus `latest` on the default branch and semantic-version tags on `v*` refs, captures `steps.build.outputs.digest`, runs `kustomize edit set image` on the runner, renders `dist/manifests/aisphere-iam.yaml`, writes `image-ref.txt`, `source-sha.txt`, contract metadata, and `SHA256SUMS`, then uploads a zip artifact. Tag runs attach the zip to the GitHub Release.

- [ ] **Step 4: Verify no deployment authority**

Run the workflow safety test and inspect `rg -n "kubectl (apply|set image|rollout)|kubeconfig|KUBE_CONFIG" .github/workflows` for zero matches.

## Task 6: Generate the IAM Front Fetch client

**Files:** `openapi/*`, `orval.config.ts`, `src/lib/api/generated/*`, `src/lib/api/iam-fetch.ts`, `src/lib/api/iam-fetch.test.ts`, `package.json`, `package-lock.json`

- [ ] **Step 1: Copy and lock the approved IAM contract**

Copy the normalized Swagger snapshot and create `contract-lock.json` with `repository`, `git_sha`, `ref`, `sha256`, `kernel_version`, and `generator` fields. Add a Node verifier that recalculates SHA-256 and fails on mismatch.

- [ ] **Step 2: Write red Fetch adapter tests**

Test that the adapter:

```text
sends credentials=include
preserves caller headers
throws Authentication required for manual redirects
throws IamApiError with status/code/requestId/traceId/metadata
returns typed JSON for successful responses
```

- [ ] **Step 3: Implement the custom mutator**

Export `iamFetch<T>(config: RequestInit & { url: string }): Promise<T>` and `IamApiError`. Reuse `apiUrl()` and the existing same-origin Envoy behavior; do not read browser token storage and do not add Axios.

- [ ] **Step 4: Configure and run Orval**

Use Orval 8 with `client: "fetch"`, `mode: "tags-split"`, the custom mutator, deterministic cleanup, and output under `src/lib/api/generated`.

- [ ] **Step 5: Add package commands**

```json
{
  "api:generate": "orval --config orval.config.ts",
  "api:check": "npm run contract:check && npm run api:generate && git diff --exit-code -- src/lib/api/generated",
  "contract:check": "node scripts/verify-contract-lock.mjs",
  "typecheck": "tsc --noEmit --pretty false",
  "test:run": "vitest run"
}
```

- [ ] **Step 6: Run adapter tests, generation, and typecheck**

```powershell
npm run contract:check
npm run api:generate
npm test -- --run src/lib/api/iam-fetch.test.ts
npm run typecheck
```

## Task 7: Migrate Group CRUD and membership as the tracer slice

**Files:** `src/lib/api/index.ts`, `src/hooks/use-iam.ts`, `src/lib/api/group-generated.test.ts`

- [ ] **Step 1: Write red contract-usage tests**

Assert Group create/update/delete and membership functions call the generated operations with `{ group }`, PATCH, recursive query, and path parameters. The test must not assert private generated implementation.

- [ ] **Step 2: Replace Group transport calls**

Keep UI-facing `iamDirectoryApi` and `useIam*` names, but delegate to generated Group functions. Delete the five hand-written Group URL constructions and `IamGroupWrite` backend DTO.

- [ ] **Step 3: Run the focused and full frontend suites**

```powershell
npm test -- --run
npm run typecheck
npm run build
```

## Task 8: Make IAM Front deliver image and digest-pinned YAML only

**Files:** `.github/workflows/build-image.yml`, `.github/workflows/deploy-k8s.yml`, `deploy/kustomization.yaml`, `scripts/verify-delivery-workflow.mjs`

- [ ] **Step 1: Write the red workflow safety test**

Require contract verification, generation diff, typecheck, Vitest, Next build, Docker build, digest capture, Kustomize render, bundle checksums, and artifact upload. Reject all cluster mutation and kubeconfig text.

- [ ] **Step 2: Remove direct deployment workflow**

Delete `deploy-k8s.yml`. Expand `build-image.yml` into verification and delivery jobs with no cluster permissions.

- [ ] **Step 3: Render the frontend bundle**

After push, pin the Deployment image to the registry digest, render `aisphere-iam-frontend.yaml`, include contract lock and metadata, generate checksums, upload on main, and attach to GitHub Release on tags.

- [ ] **Step 4: Run static and rendered-YAML checks**

Expected: no cluster mutation commands; rendered Deployment contains `@sha256:` on delivery runs and no Secret values.

## Task 9: Promote reusable OpenAPI rules to Kernel

**Files:** `cmd/openapi-contract/*`, `docs/contracts/openapi.md`, `.github/workflows/verify.yml`

- [ ] **Step 1: Port the passing IAM fixture cases into Kernel red tests**

The Kernel command accepts service title/version and the stable Kernel error schema, with no IAM package imports.

- [ ] **Step 2: Implement the generic command and documentation**

Document proto/OpenAPI ownership, direct HTTP error encoding, deterministic output, and consumer compatibility requirements.

- [ ] **Step 3: Repair Kernel PR verification**

Run against PR HEAD, test generator/checker packages, generate a temporary project, and fail on drift. Remove cross-branch file copying and automatic commit/push behavior.

- [ ] **Step 4: Verify Kernel**

```powershell
go test ./cmd/openapi-contract ./cmd/buf-check-aisphere ./cmd/protoc-gen-go-http ./transportx/http
```

IAM keeps its temporary command until a released Kernel version contains this tool; removal is a later version-bump change, not part of this plan.

## Task 10: Fresh end-to-end verification

- [ ] **Step 1: IAM verification**

```powershell
make api
make deploy
make proto-check
go test ./cmd/openapi-contract-check ./internal/server ./...
go build ./cmd/aisphere-iam
git diff --check
```

- [ ] **Step 2: IAM Front verification**

```powershell
npm ci
npm run contract:check
npm run api:generate
npm run api:check
npm run typecheck
npm run test:run
npm run build
git diff --check
```

- [ ] **Step 3: Kernel verification**

```powershell
go test ./cmd/openapi-contract ./cmd/buf-check-aisphere ./cmd/protoc-gen-go-http ./transportx/http
git diff --check
```

- [ ] **Step 4: Delivery inspection**

Render both Kustomizations with immutable test digests. Confirm all expected resources, no Secret documents, no `CHANGE_ME`, and no mutable image tags.

- [ ] **Step 5: Commit by vertical slice**

Use separate commits for generated service ownership, OpenAPI governance, frontend generated client, backend delivery, frontend delivery, and Kernel reusable tooling. Do not mix generated artifacts with unrelated cleanup.
