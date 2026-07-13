#!/bin/bash
# Run integration tests against aisphere-dev
# Usage: ssh root@36.137.200.194 'bash -s' < scripts/run-integration-tests.sh

set -euo pipefail

NAMESPACE="aisphere"
TEST_POD="iam-integration-test-runner"
IAM_SERVICE="aisphere-iam"
GIT_SHA="${1:-latest}"

echo "=== Integration Test Runner ==="
echo "Namespace: $NAMESPACE"
echo "Test pod: $TEST_POD"
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
    image: golang:1.25.12-alpine
    command: ["sleep", "3600"]
    env:
    - name: KERNEL_ENV
      value: "test"
    - name: CASDOOR_ENDPOINT
      value: "http://casdoor.aisphere:8000"
    - name: SPICEDB_ENDPOINT
      value: "spicedb.aisphere:50051"
    - name: SPICEDB_TOKEN
      value: "keykeykey"
    - name: POSTGRES_DSN
      value: "postgres://postgres:ChangeMe_PostgreSQL_123root@apps-postgre.aisphere:5432/aisphere_iam_test?sslmode=disable"
  restartPolicy: Never
EOF

echo "=== 2. Waiting for test pod ready ==="
kubectl wait --for=condition=ready "pod/$TEST_POD" -n "$NAMESPACE" --timeout=120s

echo "=== 3. Installing Go test dependencies ==="
kubectl exec "$TEST_POD" -n "$NAMESPACE" -- apk add --no-cache git

echo "=== 4. Running integration tests ==="
kubectl exec "$TEST_POD" -n "$NAMESPACE" -- sh -c '
  cd /tmp && \
  cat > /tmp/main_test.go << '\''EOF'\''
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestIntegrationPostgresConnection(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}
	t.Log("PostgreSQL connection OK")
}

func TestIntegrationCasdoorConnection(t *testing.T) {
	endpoint := os.Getenv("CASDOOR_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://casdoor.aisphere:8000"
	}
	resp, err := http.Get(endpoint + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("Casdoor HTTP: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Casdoor status: %d", resp.StatusCode)
	}
	t.Log("Casdoor connection OK")
}

func TestIntegrationSpiceDBConnection(t *testing.T) {
	endpoint := os.Getenv("SPICEDB_ENDPOINT")
	if endpoint == "" {
		endpoint = "spicedb.aisphere:50051"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("SpiceDB gRPC dial: %v", err)
	}
	defer conn.Close()
	t.Log("SpiceDB connection OK")
}

func TestIntegrationIAMHealth(t *testing.T) {
	resp, err := http.Get("http://aisphere-iam:18080/healthz")
	if err != nil {
		t.Fatalf("IAM health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("IAM health status: %d", resp.StatusCode)
	}
	t.Log("IAM health OK")
}
EOF
'

echo "=== 5. Test results ==="
kubectl logs "$TEST_POD" -n "$NAMESPACE" --tail=50

echo ""
echo "=== Integration tests complete ==="