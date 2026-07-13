#!/bin/bash
# Run integration tests against aisphere-dev (36.137.200.194)
#
# This script creates an ephemeral test pod in the K8s cluster, copies the
# integration test file into it, and runs go test with the integration tag.
#
# Usage:
#   ssh root@36.137.200.194 'bash -s' < scripts/run-integration-tests.sh
#
# Or from a machine with kubectl access to the cluster:
#   bash scripts/run-integration-tests.sh

set -euo pipefail

NAMESPACE="aisphere"
TEST_POD="iam-integration-test-runner"
IAM_SERVICE="aisphere-iam"
IAM_GRPC_ADDR="${IAM_SERVICE}.${NAMESPACE}:19080"

echo "=========================================="
echo "  IAM Integration Test Runner"
echo "=========================================="
echo "Namespace: $NAMESPACE"
echo "Test pod: $TEST_POD"
echo "IAM gRPC: $IAM_GRPC_ADDR"
echo ""

cleanup() {
  echo "=== Cleaning up test pod ==="
  kubectl delete pod "$TEST_POD" -n "$NAMESPACE" --force --grace-period=0 2>/dev/null || true
}
trap cleanup EXIT

echo "=== 1. Creating test pod ==="
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $TEST_POD
  namespace: $NAMESPACE
spec:
  containers:
  - name: tester
    image: golang:1.26-alpine
    command: ["sleep", "3600"]
    env:
    - name: IAM_GRPC_ADDR
      value: "$IAM_GRPC_ADDR"
    - name: POSTGRES_DSN
      value: "postgres://postgres:ChangeMe_PostgreSQL_123root@apps-postgre.aisphere:5432/aisphere_iam_test?sslmode=disable"
    - name: CASDOOR_ENDPOINT
      value: "http://casdoor.aisphere:8000"
    - name: SPICEDB_ENDPOINT
      value: "spicedb.aisphere:50051"
    - name: SPICEDB_TOKEN
      value: "keykeykey"
  restartPolicy: Never
EOF

echo "=== 2. Waiting for test pod ready ==="
kubectl wait --for=condition=ready "pod/$TEST_POD" -n "$NAMESPACE" --timeout=120s

echo "=== 3. Installing dependencies ==="
kubectl exec "$TEST_POD" -n "$NAMESPACE" -- apk add --no-cache git

echo "=== 4. Cloning repo and running integration tests ==="
kubectl exec "$TEST_POD" -n "$NAMESPACE" -- sh -c '
  set -e
  cd /tmp
  git clone --depth 1 https://github.com/aisphereio/aisphere-iam.git 2>/dev/null || echo "repo already cloned"
  cd aisphere-iam

  # Create a test config pointing to cluster services
  cat > configs/config.test.yaml << CONFIGEOF
service:
  name: iam-service
  version: test
  env: test
server:
  http:
    addr: 0.0.0.0:18080
    timeout_ns: 5000000000
  grpc:
    addr: 0.0.0.0:19080
    timeout_ns: 5000000000
log:
  service_name: iam-service
  env: test
  version: test
  level: debug
  format: console
  output: stdout
  add_source: true
  redact:
    enabled: true
    keys: [password, secret, token, access_key, secret_key, client_secret]
    value: "***"
  access_log:
    enabled: false
data:
  database:
    enabled: true
    config:
      auto_create_database: false
      driver: postgres
      dsn: "${POSTGRES_DSN}"
      max_open_conns: 10
      max_idle_conns: 5
      conn_max_lifetime_ns: 1800000000000
      conn_max_idle_time_ns: 300000000000
      query_timeout_ns: 5000000000
      slow_query_threshold_ns: 200000000
      audit_enabled: true
      metrics_enabled: false
  migration:
    enabled: true
    config:
      enabled: true
      engine: goose
      dir: migrations
      table: iam_schema_migrations
      mode: apply
      fail_on_pending: true
      allow_concurrent: false
  cache:
    enabled: false
  object_store:
    enabled: false
security:
  authn:
    enabled: true
    mode: gateway_trusted
    provider: casdoor
    cache_ttl_ns: 300000000000
    oidc:
      provider: casdoor
      issuer: http://casdoor.aisphere:8000
      discovery_url: http://casdoor.aisphere:8000/.well-known/openid-configuration
      jwks_url: http://casdoor.aisphere:8000/.well-known/jwks
      audience: [bbdcfc272e2b990cb923]
      allowed_owners: [aisphere]
      allowed_algs: [RS256, RS512, ES256, ES512]
      jwks_cache_ttl_ns: 600000000000
      clock_skew_ns: 60000000000
    casdoor:
      endpoint: http://casdoor.aisphere:8000
      organization_name: aisphere
      application_name: aisphere
      client_id: "bbdcfc272e2b990cb923"
      client_secret: "c4d351406d40251b267624328e50e8a1f7352a65"
      jwt_certificate_file: ./configs/casdoor.pub
      timeout: 10000000000
      metrics_enabled: false
      admin:
        enabled: true
        organization_name: aisphere
        application_name: iam-service
        client_id: "869aff97ab0408cbbd1c"
        client_secret: "dbd4c663c6dbd947da0d23616cddaf532dcee7be"
        jwt_certificate_file: ./configs/casdoor.pub
  internal_call:
    enabled: false
  authz:
    enabled: true
    provider: spicedb
    dev_allow_all: false
    install_default_schema: true
    schema_path: configs/spicedb/aisphere.schema.zed
    spicedb:
      endpoint: spicedb.aisphere:50051
      token: "${SPICEDB_TOKEN}"
      transport: grpc
      insecure: true
      timeout: 5000000000
      fully_consistent: true
      metrics_enabled: false
control_plane:
  defaults:
    enabled: true
    path: configs/resource/defaults.yaml
  bootstrap_admins:
    enabled: true
    subjects:
      - type: user
        username: admin
        zone_id: aisphere
        casdoor_org: aisphere
        role: zone_owner
        source: bootstrap
        reason: initial Casdoor admin user
audit:
  enabled: true
  store: memory
metrics:
  enabled: false
dtm:
  enabled: false
CONFIGEOF

  echo "=== Running integration tests ==="
  go test -tags=integration ./internal/service/ -run TestIntegration -v -count=1 2>&1
  EXIT_CODE=$?
  echo ""
  echo "=== Exit code: $EXIT_CODE ==="
  exit $EXIT_CODE
'

echo ""
echo "=== Integration tests complete ==="