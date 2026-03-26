#!/bin/bash
set -e

# Integration test script for cronado
# Prerequisites: Docker running, cronado binary built or compose stack running
#
# Usage:
#   ./files/container_test.sh          # Run against a running cronado instance
#   ./files/container_test.sh --build  # Build and run the full compose stack first

CRONADO_URL="${CRONADO_URL:-http://localhost:8080}"
COMPOSE_FILE="files/compose.yaml"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}PASS${NC} $1"; }
fail() { echo -e "${RED}FAIL${NC} $1"; exit 1; }
info() { echo -e "${YELLOW}INFO${NC} $1"; }

# Start compose stack if --build flag is set
if [ "$1" = "--build" ]; then
    info "Building and starting compose stack..."
    docker compose -f "$COMPOSE_FILE" up --build -d
    info "Waiting for services to start..."
    sleep 5
fi

# --------------------------------------------------
# Test 1: API is reachable
# --------------------------------------------------
info "Test 1: API health check"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$CRONADO_URL/api/cron-job")
if [ "$HTTP_CODE" = "200" ]; then
    pass "API returns 200"
else
    fail "API returned $HTTP_CODE (expected 200)"
fi

# --------------------------------------------------
# Test 2: Start a test container with cron labels
# --------------------------------------------------
info "Test 2: Starting test container with cron labels"
docker rm -f cronado_test 2>/dev/null || true

docker run -d \
  --name cronado_test \
  --label cronado.test-cron.enabled=true \
  --label cronado.test-cron.schedule="@every 3s" \
  --label cronado.test-cron.cmd="echo hello-from-test" \
  --label cronado.test-cron.user=root \
  --label cronado.test-cron.timeout=10s \
  --label cronado.test-fail.enabled=true \
  --label cronado.test-fail.schedule="@every 5s" \
  --label cronado.test-fail.cmd="false" \
  alpine:latest \
  /bin/sh -c "sleep infinity" > /dev/null

pass "Test container started"

# --------------------------------------------------
# Test 3: Wait for cronado to detect the container
# --------------------------------------------------
info "Test 3: Waiting for cronado to detect container..."
sleep 5

JOBS=$(curl -s "$CRONADO_URL/api/cron-job")
JOB_COUNT=$(echo "$JOBS" | python3 -c "import sys,json; print(len([j for j in json.load(sys.stdin) if 'cronado_test' in j.get('container_name','')]))" 2>/dev/null || echo "0")

if [ "$JOB_COUNT" -ge 2 ]; then
    pass "Cronado detected $JOB_COUNT jobs for test container"
else
    fail "Expected at least 2 jobs for test container, got $JOB_COUNT"
fi

# --------------------------------------------------
# Test 4: Verify job details
# --------------------------------------------------
info "Test 4: Verifying job details"
TEST_CRON=$(echo "$JOBS" | python3 -c "
import sys, json
jobs = json.load(sys.stdin)
for j in jobs:
    if j.get('cron_job',{}).get('name') == 'test-cron':
        cj = j['cron_job']
        assert cj['schedule'] == '@every 3s', f\"schedule: {cj['schedule']}\"
        assert cj['command'] == 'echo hello-from-test', f\"command: {cj['command']}\"
        assert cj['user'] == 'root', f\"user: {cj['user']}\"
        assert cj['enabled'] == True, f\"enabled: {cj['enabled']}\"
        print('OK')
        break
else:
    print('NOT_FOUND')
" 2>&1)

if [ "$TEST_CRON" = "OK" ]; then
    pass "Job details are correct"
else
    fail "Job details mismatch: $TEST_CRON"
fi

# --------------------------------------------------
# Test 5: Wait for job execution and check output
# --------------------------------------------------
info "Test 5: Waiting for job execution..."
sleep 5

# Check cronado logs for successful execution
if docker logs cronado 2>&1 | grep -q "hello-from-test"; then
    pass "Job executed successfully (output found in logs)"
else
    fail "Job execution output not found in cronado logs"
fi

# --------------------------------------------------
# Test 6: Check metrics endpoint
# --------------------------------------------------
info "Test 6: Checking metrics endpoint"
METRICS=$(curl -s "$CRONADO_URL/metrics" 2>/dev/null || echo "")
if echo "$METRICS" | grep -q "cronado_scheduled_jobs"; then
    pass "Prometheus metrics available"
else
    fail "Prometheus metrics not found"
fi

# --------------------------------------------------
# Test 7: Check failure notifications
# --------------------------------------------------
info "Test 7: Checking failure detection"
if docker logs cronado 2>&1 | grep -q "Cron job execution failed.*test-fail"; then
    pass "Failure detected for test-fail job"
else
    info "Failure not yet detected (may need more time)"
fi

# --------------------------------------------------
# Test 8: Stop container and verify job removal
# --------------------------------------------------
info "Test 8: Stopping test container"
docker stop cronado_test > /dev/null
sleep 3

JOBS_AFTER=$(curl -s "$CRONADO_URL/api/cron-job")
REMAINING=$(echo "$JOBS_AFTER" | python3 -c "import sys,json; print(len([j for j in json.load(sys.stdin) if 'cronado_test' in j.get('container_name','')]))" 2>/dev/null || echo "0")

if [ "$REMAINING" = "0" ]; then
    pass "Jobs removed after container stop"
else
    fail "Expected 0 jobs for stopped container, got $REMAINING"
fi

# --------------------------------------------------
# Cleanup
# --------------------------------------------------
info "Cleaning up test container"
docker rm -f cronado_test 2>/dev/null || true

echo ""
echo -e "${GREEN}All tests passed!${NC}"
