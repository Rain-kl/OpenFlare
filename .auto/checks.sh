#!/bin/bash
# Correctness gate: must pass after every edit. Fails fast on real breakage.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==> go vet ./..."
go vet ./... 2>&1 | tail -20

echo "==> go build ./..."
go build ./... 2>&1 | tail -20

echo "==> golangci-lint run (repo config)"
golangci-lint run 2>&1 | tail -20

# 全量单测（sqlite + miniredis，纯本地无需外部服务；2026-08-16 起全绿）
echo "==> go test ./internal/... ./pkg/..."
go test ./internal/... ./pkg/... 2>&1 | grep -E "^--- FAIL|^FAIL" | head -20 || true
if go test ./internal/... ./pkg/... > /tmp/auto_gotest.log 2>&1; then
  :
else
  tail -30 /tmp/auto_gotest.log
  exit 1
fi

# 前端测试（vitest；2026-08-16 起全绿）
echo "==> pnpm exec vitest run (frontend)"
(cd frontend && pnpm exec vitest run --reporter=dot > /tmp/auto_vitest.log 2>&1) || {
  tail -30 /tmp/auto_vitest.log
  exit 1
}

echo "OK: checks passed"