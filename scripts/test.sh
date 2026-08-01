#!/usr/bin/env bash
# Run all Go tests (unit + integration) and the backend self-test.
set -euo pipefail
cd "$(dirname "$0")/.."

go test ./...
go build -o /tmp/openinfer-core-selftest ./apps/core
OUT=$(/tmp/openinfer-core-selftest --selftest --token ci-token --data-dir /tmp/openinfer-selftest-data)
echo "$OUT" | grep -q '"ready":true' || { echo "selftest failed: $OUT"; exit 1; }
rm -rf /tmp/openinfer-selftest-data /tmp/openinfer-core-selftest
echo "ALL TESTS PASSED"
