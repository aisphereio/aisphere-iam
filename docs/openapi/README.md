# IAM OpenAPI contracts

The IAM HTTP API is defined by protobuf under `api/**/*.proto`. OpenAPI is a generated, versioned contract and must not be edited by hand.

## Files

- `iam.full.swagger.json`: all protobuf-declared HTTP operations, including internal service-to-service operations.
- `iam.console.swagger.json`: only `PUBLIC`, `AUTHENTICATED`, and `AUTHORIZED` operations. This is the contract consumed by the IAM frontend.
- `openapi.lock.json`: deterministic SHA-256 hashes for both contracts.
- `aisphere.swagger.json`: temporary merged output from `protoc-gen-openapiv2`; intentionally ignored by Git.

## Commands

```bash
make tools
make openapi
make openapi-check
```

`make openapi` performs protobuf generation first, then derives the full and console contracts. The console contract is filtered from the access-policy `exposure` declared on each RPC. An HTTP RPC without an access exposure fails generation.

`make openapi-check` regenerates the temporary Swagger document and verifies that all committed contract files are byte-for-byte current.

## Runtime endpoints

The IAM image includes the generated files and serves them at:

- `/openapi/iam/full.swagger.json`
- `/openapi/iam/console.swagger.json`
- `/openapi/iam/contract.json`

The full contract should remain protected by gateway policy. The console contract is the frontend integration surface.

## Release lifecycle

When a GitHub Release is published, `.github/workflows/openapi-release.yml`:

1. checks out the exact release tag;
2. regenerates and verifies the contracts;
3. uploads both specifications, the lock file, a release manifest, and a compressed bundle to that release;
4. optionally dispatches `iam-openapi-released` to `aisphereio/aisphere-iam-frontend`.

Cross-repository dispatch requires the backend repository secret `IAM_FRONTEND_SYNC_TOKEN`. The token needs permission to create repository dispatch events in the frontend repository.

The frontend records the backend tag, commit and contract SHA in its own lock file. Rollback therefore restores both the backend image and its matching generated frontend client.
