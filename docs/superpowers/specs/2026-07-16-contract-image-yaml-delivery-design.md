# Contract, Image, and Deployment YAML Delivery Design

## Goal

Make GitHub Actions the authoritative build and release path for Aisphere IAM while keeping the existing Kernel proto-first model:

1. IAM proto remains the only business API source of truth;
2. the generated OpenAPI contract becomes versioned, validated, and consumable by frontend code generation;
3. IAM and IAM Front publish immutable container images;
4. each release publishes fully rendered deployment YAML pinned to the exact image digest;
5. GitHub Actions never stores cluster credentials and never executes `kubectl apply`.

The production deliverables are image references and deployment bundles. Applying those bundles to a cluster belongs to a separate operations process.

## Scope decomposition

This program spans three repositories and is implemented as four independently verifiable slices:

1. **IAM contract authority** in `aisphere-iam`;
2. **IAM Front generated client** in `aisphere-iam-front`;
3. **image and YAML delivery** in both deployable repositories;
4. **shared contract rules** in `kernel`, released before IAM removes any temporary service-local compatibility logic.

Each slice must remain releasable on its own. IAM must not consume an unreleased Kernel command from a production GitHub Action.

## Current state

### Kernel and IAM generation

`aisphere-iam/make api` already uses Kernel generators plus grpc-gateway tooling to produce Go protobuf messages, gRPC bindings, direct HTTP bindings, access resolvers, Gateway manifests, Kernel service modules, and one merged Swagger 2.0 file.

The generated OpenAPI file is currently written to `docs/openapi/aisphere.swagger.json`, but `docs/openapi/` is ignored by Git. IAM CI regenerates code and checks the Git diff, so it detects stale committed Go and deployment files but cannot detect OpenAPI drift.

`buf.yaml` selects `FILE` breaking rules, but no Make target or GitHub workflow executes `buf breaking`.

Generated service coverage is incomplete at runtime. `IAMDirectoryProjectionService` and `IAMAuthorizationAdminService` have generated modules but are not included in `IAMModules()`. The authorization-admin service and directory-projection routes therefore retain manual registration or compatibility handlers beside the generated catalog.

### OpenAPI accuracy

The current Swagger document describes 52 paths, 69 operations, and 106 definitions. Operation IDs and service tags are present, but most operations have no summary or description.

The HTTP error schema is inaccurate. Swagger describes the default response as `rpcStatus`, while Kernel direct HTTP transport returns the stable `errorx` representation:

```json
{
  "code": "IAM_PERMISSION_DENIED",
  "message": "permission denied",
  "request_id": "req_...",
  "trace_id": "trace_...",
  "metadata": {}
}
```

Some expanded optional request filters are also marked as required in Swagger. These contract defects must be fixed or normalized before generated clients become authoritative.

### IAM Front

IAM Front uses npm, Fetch, TanStack Query, and Envoy Gateway OIDC cookies. It does not store access tokens in browser storage. Its request module correctly supplies `credentials: include` and handles Gateway redirects.

Backend DTOs, paths, methods, and query parameters remain hand-written. The repository contains approximately 52 literal IAM paths, 29 exported IAM contract types, and broad response normalizers that accept several historical JSON shapes. Existing drift includes query-name differences, flattened filters where the contract is nested, int64 values represented as JavaScript numbers instead of JSON strings, and authorization write paths that are not part of the authorization-admin proto surface.

The existing `use-iam.ts` and `use-authz.ts` modules contain valuable UI behavior: stable query keys, enablement rules, invalidation, and domain-specific transformations. They are not transport contract modules and should not be deleted in the first migration.

### Delivery

IAM has duplicate image workflows. IAM Front has one image workflow and a separate workflow that holds kubeconfig credentials and directly applies manifests. Deployment kustomizations use mutable `latest` tags. IAM's base kustomization does not currently include every generated Gateway route in one rendered output.

## Considered approaches

### OpenAPI-first rewrite

Move route and schema ownership from proto to a hand-maintained OpenAPI document, then generate Go and TypeScript from it.

Rejected. It would duplicate and weaken the existing Kernel proto policy, Gateway, access, audit, and service-module generation chain.

### Browser gRPC or Connect migration

Generate TypeScript protobuf clients and replace browser REST calls with gRPC-Web or Connect.

Rejected. It introduces another Gateway adapter and authentication path without solving the current contract-governance problem. REST/JSON is already the supported external protocol.

### Versioned OpenAPI plus per-consumer generated Fetch clients

Keep proto as the source, normalize and publish the generated OpenAPI artifact, and let each frontend generate a Fetch client using its own transport adapter.

Selected. It preserves Kernel ownership, supports the existing Envoy cookie flow, and avoids coupling UI caching behavior to a shared package prematurely. Once IAM Front and Hub Front converge on the same transport semantics, a shared npm client can be published without changing the contract source.

## Architecture

### Ownership

#### Kernel

Kernel owns reusable contract rules and transport truth:

- proto access-policy validation;
- generated HTTP, Gateway, AuthZ, and service-module glue;
- the stable `errorx` HTTP response shape;
- generic OpenAPI normalization and validation behavior;
- generated-project smoke coverage in Kernel CI.

Kernel does not own IAM business proto files or IAM's published OpenAPI artifact.

#### IAM backend

IAM owns:

- IAM proto contracts;
- deterministic generated artifacts;
- the versioned IAM OpenAPI document;
- IAM-specific operation documentation;
- compatibility gates against the target branch;
- image and backend deployment-bundle publication.

All public business `/v1/iam` routes must come from generated proto bindings. DTM branch callbacks remain explicit internal adapters. UI login redirects belong to Gateway/UI routing rather than the IAM business SDK. Casdoor webhook exposure must either become a declared proto route or be explicitly classified as an internal adapter before the generated catalog is considered complete.

#### IAM Front

IAM Front owns:

- the pinned OpenAPI snapshot and its source lock;
- Orval configuration;
- generated Fetch functions and backend DTOs;
- the hand-written Envoy OIDC/error transport adapter;
- UI domain models and TanStack Query behavior;
- frontend image and deployment-bundle publication.

Generated files are never edited manually.

### Contract data flow

```text
IAM .proto
  -> buf lint
  -> buf-check-aisphere
  -> buf breaking against target branch
  -> buf generate
  -> Kernel/IAM OpenAPI normalize + validate
  -> committed and released aisphere.swagger.json
  -> frontend pinned snapshot + source SHA + SHA-256
  -> Orval Fetch client and DTO generation
  -> existing domain hook facade
  -> IAM UI
```

The frontend never downloads a contract from a running IAM environment. Local sync uses an adjacent IAM checkout or an explicit immutable Git commit/tag. CI regenerates only from the pinned snapshot.

### OpenAPI governance

The IAM repository will stop ignoring `docs/openapi/` and commit the normalized Swagger artifact. Generation must be deterministic.

The normalized document must contain:

- stable title and version metadata;
- one unique operation ID for every operation;
- non-empty tags;
- correct request requiredness;
- the Kernel HTTP error schema rather than `rpcStatus` for direct HTTP endpoints;
- no public route that bypasses the generated route catalog;
- no duplicate method/path pair.

Compatibility is checked at two levels:

1. `buf breaking` protects protobuf source and wire compatibility;
2. an OpenAPI semantic diff protects HTTP path, method, parameter, request, and response compatibility.

The OpenAPI diff is required because protobuf breaking checks do not govern changes inside `google.api.http` custom options.

### Frontend generation seam

Orval uses the native Fetch client. The generated code receives one custom mutator that delegates to the existing same-origin request behavior and throws a structured `IamApiError` on non-success responses.

`IamApiError` retains:

- HTTP status;
- stable Kernel error code;
- safe message;
- request ID;
- trace ID;
- public metadata.

The first migration keeps `use-iam.ts` and `use-authz.ts` as the UI-facing interface. Each vertical slice replaces hand-written transport calls and DTOs with generated functions, then removes obsolete normalization code. Group CRUD and membership are the tracer slice because their proto-generated routes and request bodies are already covered by backend contract tests.

### Contract distribution

Every IAM tag release attaches:

```text
aisphere-iam-contract-<version>.zip
  openapi/aisphere.swagger.json
  contract-lock.json
  SHA256SUMS
```

`contract-lock.json` records the IAM Git SHA, release ref, OpenAPI SHA-256, generator versions, and Kernel version.

IAM Front commits its selected snapshot and a consumer lock. A manual GitHub Action accepts an immutable IAM commit or tag, updates the snapshot and generated client on a branch, runs all consumer checks, and opens a pull request. It never tracks a moving deployed endpoint.

Automatic cross-repository dispatch is deferred until a repository-scoped automation token is deliberately provisioned. The correctness of normal builds does not depend on that token.

## Image and deployment YAML delivery

### Immutable image identity

IAM and IAM Front continue publishing to the existing Aliyun registry. Release workflows always publish a `sha-<short-sha>` tag and capture the registry digest. Human-friendly branch, semantic-version, and `latest` tags may also be published, but deployment YAML must reference the digest:

```text
registry.cn-beijing.aliyuncs.com/ainfracn/aisphere-iam@sha256:...
registry.cn-beijing.aliyuncs.com/ainfracn/aisphere-iam-frontend@sha256:...
```

### Rendered bundle

After the image is pushed, the workflow renders Kustomize with the exact digest and produces:

```text
aisphere-iam-delivery-<version>.zip
  manifests/aisphere-iam.yaml
  metadata/image-ref.txt
  metadata/source-sha.txt
  openapi/aisphere.swagger.json
  SHA256SUMS

aisphere-iam-frontend-delivery-<version>.zip
  manifests/aisphere-iam-frontend.yaml
  metadata/image-ref.txt
  metadata/source-sha.txt
  metadata/iam-contract-lock.json
  SHA256SUMS
```

The backend rendered YAML includes namespace-scoped application resources and generated Gateway API routes. Secret values are never rendered or packaged; manifests reference externally managed Secret objects. Placeholder `secret.yaml` remains an example only and is not a Kustomize resource.

Pull requests build images without pushing and render preview YAML using a non-deployable test image reference. Main and tag workflows push images. Tag workflows attach durable bundles to the GitHub Release; main workflows upload short-lived Actions artifacts for verification.

### No deployment authority

Delivery workflows have no kubeconfig secret, no cloud cluster credential, and no `kubectl apply`, `kubectl set image`, or rollout step. The existing IAM Front direct-deploy workflow is retired. Local Make targets may render or validate manifests, but direct cluster mutation is outside the supported release path.

## GitHub Actions topology

### Kernel verification

Kernel pull requests run tests for contract checkers, generators, `errorx` HTTP encoding, and generated-project smoke fixtures against the checked-out PR commit. Kernel workflows must not fetch implementation files from another branch or commit generated changes back to the repository.

### IAM verification and delivery

The IAM workflow performs:

```text
checkout with full history
-> install pinned Go/Buf/Kernel tools
-> contract checks
-> deterministic API and Gateway generation
-> OpenAPI validation and compatibility diff
-> Go tests, traceability, and build
-> container build
-> render and validate deployment YAML
-> generated-worktree diff check
-> publish image and bundles only on main/tag
```

The duplicate legacy image workflow is removed.

### IAM Front verification and delivery

The IAM Front workflow performs:

```text
npm ci
-> verify contract lock
-> regenerate Orval output
-> generated-worktree diff check
-> TypeScript check
-> Vitest
-> Next production build
-> container build
-> render and validate deployment YAML
-> publish image and bundles only on main/tag
```

## Failure behavior

- A stale generated OpenAPI or client diff blocks the pull request.
- A protobuf or HTTP breaking change blocks the pull request unless the contract versioning policy explicitly approves it.
- A missing operation ID, incorrect Kernel error schema, public manual route, unresolved reference, or duplicate route blocks contract publication.
- A frontend contract digest mismatch blocks generation and image publication.
- A failed typecheck, test, image build, Kustomize render, or YAML validation blocks delivery.
- A registry login or push failure produces no deployment bundle, preventing a YAML artifact from referring to an unavailable image.
- Release jobs never fall back to mutable `latest` in rendered YAML.

## Testing seams

Implementation follows vertical red-green cycles at these public seams:

1. `make contract-check` for the IAM contract toolchain;
2. `IAMModules()` and the generated HTTP catalog for route ownership;
3. the frontend generated client plus custom Fetch mutator for request and error behavior;
4. `make render` / delivery scripts for digest-pinned YAML;
5. GitHub workflow static assertions for permissions, triggers, required jobs, and absence of cluster mutation commands.

Tests must observe these interfaces rather than private helper implementation.

## Acceptance criteria

1. A proto change updates Go, Gateway, OpenAPI, and frontend-generated surfaces through documented commands.
2. IAM CI rejects stale OpenAPI and protobuf/HTTP breaking changes.
3. IAM Front CI rejects stale generated clients and contract-lock mismatches.
4. Group CRUD and membership no longer hand-write backend paths or request DTOs.
5. Runtime errors reach the frontend as structured `IamApiError` values.
6. Every public IAM business route is generated or explicitly rejected by the contract checker.
7. IAM and IAM Front release workflows publish images with immutable SHA tags and digests.
8. Delivered YAML references image digests, includes generated Gateway routes where applicable, and contains no secret values.
9. No GitHub Action contains cluster credentials or executes cluster mutations.
10. Tag releases contain durable contract and deployment bundles with SHA-256 checksums.

## Non-goals

- migrating browser traffic to gRPC-Web or Connect;
- switching from npm to pnpm;
- replacing Envoy Gateway OIDC with browser token storage;
- auto-applying manifests to Kubernetes;
- publishing a shared React Query hook package in the first migration;
- migrating Hub Front in the first implementation plan.
