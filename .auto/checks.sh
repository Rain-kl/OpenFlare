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

echo "OK: checks passed"