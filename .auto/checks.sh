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
(cd frontend && node scripts/merge-i18n-fragments.mjs && pnpm exec vitest run --reporter=dot > /tmp/auto_vitest.log 2>&1) || {
  tail -30 /tmp/auto_vitest.log
  exit 1
}

# SPDX license 头门禁（repo 自带约定）
echo "==> make license-check"
make license-check 2>&1 | grep "needs license" | head -10 || true
if make license-check > /tmp/auto_license.log 2>&1; then
  :
else
  tail -15 /tmp/auto_license.log
  exit 1
fi

# 并发密集包 -race 门禁（2026-08-16 全仓 -race 清零后纳入，防回归；
# frpc/frps 慢套件不含在此，另做全量周期验证）
echo "==> go test -race (concurrency packages)"
RACE_PKGS="./internal/apps/oauth/ ./internal/apps/openflare/tls/ ./internal/apps/openflare/uptimekuma/ ./internal/apps/upload/cache/ ./internal/repository/ ./pkg/cache/disk/ ./pkg/logger/ ./internal/infra/persistence/batchwriter/"
if go test -race -count=1 $RACE_PKGS > /tmp/auto_race.log 2>&1; then
  :
else
  grep -E "WARNING: DATA RACE|^--- FAIL|^FAIL" /tmp/auto_race.log | head -20
  exit 1
fi

echo "OK: checks passed"