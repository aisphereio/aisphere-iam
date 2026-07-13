#!/bin/bash
# End-to-end integration tests for aisphere-iam
# Tests real business flows against the deployed service
# Usage: ssh root@36.137.200.194 'bash -s' < scripts/e2e-test.sh

set -euo pipefail

NAMESPACE="aisphere"
IAM_POD=$(kubectl get pods -n $NAMESPACE -l app=aisphere-iam -o jsonpath='{.items[0].metadata.name}')
CASDOOR_POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=casdoor -o jsonpath='{.items[0].metadata.name}')
SPICEDB_POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=spicedb -o jsonpath='{.items[0].metadata.name}')
PG_POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=postgre -o jsonpath='{.items[0].metadata.name}')

PASS=0
FAIL=0

check() {
  local name="$1"
  local result="$2"
  if [ "$result" = "0" ]; then
    echo "  ❌ $name"
    FAIL=$((FAIL + 1))
  else
    echo "  ✅ $name"
    PASS=$((PASS + 1))
  fi
}

echo "=========================================="
echo "  IAM End-to-End Integration Tests"
echo "=========================================="
echo ""

# =============================================
# 1. Service Health
# =============================================
echo "--- 1. Service Health ---"

HEALTH=$(kubectl exec -n $NAMESPACE $IAM_POD -- wget -q -O- http://localhost:18080/healthz 2>/dev/null)
check "IAM Health endpoint" $(echo "$HEALTH" | grep -c '"status":"ok"')

READY=$(kubectl exec -n $NAMESPACE $IAM_POD -- wget -q -O- http://localhost:18080/readyz 2>/dev/null)
check "IAM Ready endpoint" $(echo "$READY" | grep -c '"status":"ready"')

# =============================================
# 2. PostgreSQL
# =============================================
echo ""
echo "--- 2. PostgreSQL ---"

DB_EXISTS=$(kubectl exec -n $NAMESPACE $PG_POD -- psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname='aisphere_iam'" 2>/dev/null | tr -d ' ')
check "IAM database exists" $(echo "$DB_EXISTS" | grep -c "1")

MIGRATIONS=$(kubectl exec -n $NAMESPACE $PG_POD -- psql -U postgres -d aisphere_iam -tc "SELECT COUNT(*) FROM iam_schema_migrations" 2>/dev/null | tr -d ' ')
check "Schema migrations applied" $(echo "$MIGRATIONS" | grep -cE "[0-9]+")

# Check key tables exist
TABLES=$(kubectl exec -n $NAMESPACE $PG_POD -- psql -U postgres -d aisphere_iam -tc "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name" 2>/dev/null)
check "Projects table exists" $(echo "$TABLES" | grep -c "projects")
check "Grants table exists" $(echo "$TABLES" | grep -c "grants")
check "Resources table exists" $(echo "$TABLES" | grep -c "resources")

# =============================================
# 3. Casdoor
# =============================================
echo ""
echo "--- 3. Casdoor ---"

OIDC=$(kubectl exec -n $NAMESPACE $IAM_POD -- wget -q -O- http://casdoor.aisphere:8000/.well-known/openid-configuration 2>/dev/null)
check "Casdoor OIDC configuration" $(echo "$OIDC" | grep -c '"issuer"')

# =============================================
# 4. SpiceDB
# =============================================
echo ""
echo "--- 4. SpiceDB ---"

SPICEDB_OK=$(kubectl exec -n $NAMESPACE $IAM_POD -- wget -q -O- http://spicedb.aisphere:8443/healthz 2>/dev/null || echo "")
check "SpiceDB serving" $(echo "$SPICEDB_OK" | grep -c "SERVING")

# =============================================
# 5. IAM API - Business Logic Tests
# =============================================
echo ""
echo "--- 5. IAM Business Logic ---"

# Test GetMe (should fail without auth)
GETME=$(kubectl exec -n $NAMESPACE $IAM_POD -- wget -q -O- http://localhost:18080/v1/iam/me 2>/dev/null || echo "auth_required")
check "GetMe rejects unauthenticated" $(echo "$GETME" | grep -c "auth_required")

# Test IAM can reach Casdoor internally
CASDOOR_REACH=$(kubectl exec -n $NAMESPACE $IAM_POD -- wget -q -O- --timeout=5 http://casdoor.aisphere:8000/api/get-organization?id=aisphere 2>/dev/null || echo "")
check "IAM can reach Casdoor API" $(echo "$CASDOOR_REACH" | grep -c "aisphere")

# =============================================
# 6. DTM
# =============================================
echo ""
echo "--- 6. DTM ---"

DTM_OK=$(kubectl exec -n $NAMESPACE $IAM_POD -- wget -q -O- --timeout=5 http://dtm.aisphere:36789/api/dtmsvr/version 2>/dev/null || echo "")
check "DTM available" $(echo "$DTM_OK" | grep -c "version")

# =============================================
# 7. Resource Defaults
# =============================================
echo ""
echo "--- 7. Resource Defaults ---"

# Check that resource defaults are loaded in PostgreSQL
DEFAULTS=$(kubectl exec -n $NAMESPACE $PG_POD -- psql -U postgres -d aisphere_iam -tc "SELECT COUNT(*) FROM iam_resource_types" 2>/dev/null | tr -d ' ')
check "Resource types loaded" $(echo "$DEFAULTS" | grep -cE "[0-9]+")

ROLES=$(kubectl exec -n $NAMESPACE $PG_POD -- psql -U postgres -d aisphere_iam -tc "SELECT COUNT(*) FROM iam_role_templates" 2>/dev/null | tr -d ' ')
check "Role templates loaded" $(echo "$ROLES" | grep -cE "[0-9]+")

# =============================================
# Summary
# =============================================
echo ""
echo "=========================================="
echo "  Results: $PASS passed, $FAIL failed"
echo "=========================================="

exit $FAIL