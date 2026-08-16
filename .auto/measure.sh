#!/bin/bash
# Benchmark: total code-quality issues across backend + frontend (lower is better).
# Fixed linter set — see .auto/prompt.md. Never tune this file to game counts.
set -euo pipefail
cd "$(dirname "$0")/.."
start=$(date +%s)

# ---------- Backend: golangci-lint, repo config + fixed best-practice extras ----------
EXTRA_LINTERS="errorlint,errname,nilnil,forcetypeassert,copyloopvar,intrange,mirror,perfsprint,prealloc,usestdlibvars,modernize,sloglint,canonicalheader,nosprintfhostport,recvcheck,wastedassign,exhaustive"
golang_out=$(golangci-lint run --enable="$EXTRA_LINTERS" 2>&1 || true)

golang_total=0
while IFS= read -r line; do
  if [[ "$line" =~ ^\*\ ([a-zA-Z0-9_]+):\ ([0-9]+)$ ]]; then
    name="${BASH_REMATCH[1]}"
    n="${BASH_REMATCH[2]}"
    golang_total=$((golang_total + n))
    echo "METRIC golint_${name}=$n"
  fi
done <<< "$golang_out"
echo "METRIC golint_total=$golang_total"

# ---------- Backend: test-code quality (tests excluded from repo config; safe linters only) ----------
test_out=$(golangci-lint run --tests=true --enable=testifylint,usetesting,thelper --enable-only=testifylint,usetesting,thelper 2>&1 || true)
golang_test_total=0
while IFS= read -r line; do
  if [[ "$line" =~ ^\*\ ([a-zA-Z0-9_]+):\ ([0-9]+)$ ]]; then
    name="${BASH_REMATCH[1]}"
    n="${BASH_REMATCH[2]}"
    golang_test_total=$((golang_test_total + n))
    echo "METRIC golint_test_${name}=$n"
  fi
done <<< "$test_out"
echo "METRIC golint_test_total=$golang_test_total"

# ---------- Backend: govet extra analyzers (dead code / nil deref — real-bug finders) ----------
cat > /tmp/govetx.yml <<'EOF'
version: "2"
linters:
  default: none
  enable:
    - govet
  settings:
    govet:
      enable:
        - nilness
        - unusedwrite
EOF
vetx_out=$(golangci-lint run --config /tmp/govetx.yml --max-issues-per-linter=0 2>&1 || true)
rm -f /tmp/govetx.yml
golang_vetx_total=0
while IFS= read -r line; do
  if [[ "$line" =~ ^\*\ ([a-zA-Z0-9_]+):\ ([0-9]+)$ ]]; then
    name="${BASH_REMATCH[1]}"
    n="${BASH_REMATCH[2]}"
    golang_vetx_total=$((golang_vetx_total + n))
    echo "METRIC golint_vetx_${name}=$n"
  fi
done <<< "$vetx_out"
echo "METRIC golint_vetx_total=$golang_vetx_total"

# ---------- Frontend: eslint (repo gate) ----------
cd frontend
eslint_out=$(pnpm exec eslint . --max-warnings 0 2>&1 || true)
eslint_problems=0; eslint_errors=0; eslint_warnings=0
if [[ "$eslint_out" =~ ([0-9]+)\ problems? ]]; then eslint_problems="${BASH_REMATCH[1]}"; fi
if [[ "$eslint_out" =~ \(([0-9]+)\ errors?, ]]; then eslint_errors="${BASH_REMATCH[1]}"; fi
if [[ "$eslint_out" =~ ,\ ([0-9]+)\ warnings? ]]; then eslint_warnings="${BASH_REMATCH[1]}"; fi
echo "METRIC eslint_problems=$eslint_problems"
echo "METRIC eslint_errors=$eslint_errors"
echo "METRIC eslint_warnings=$eslint_warnings"

# ---------- Frontend: tsc (repo gate) ----------
tsc_out=$(pnpm exec tsc --noEmit --jsx preserve 2>&1 || true)
tsc_errors=$(grep -cE "error TS" <<< "$tsc_out" || true)
echo "METRIC tsc_errors=$tsc_errors"

# ---------- Frontend: vitest (2026-08-16 起全绿，纳入基准防回归) ----------
vitest_out=$(pnpm exec vitest run --reporter=dot 2>&1 || true)
vitest_failed=0; vitest_total=0
if [[ "$vitest_out" =~ ([0-9]+)\ failed ]]; then vitest_failed="${BASH_REMATCH[1]}"; fi
if [[ "$vitest_out" =~ Tests[[:space:]]+([0-9]+)\ passed ]]; then vitest_total="${BASH_REMATCH[1]}"; fi
if [[ "$vitest_out" =~ Tests[[:space:]]+([0-9]+) ]]; then vitest_total="${BASH_REMATCH[1]}"; fi
echo "METRIC vitest_failed=$vitest_failed"
echo "METRIC vitest_total=$vitest_total"

end=$(date +%s)
total=$((golang_total + golang_test_total + golang_vetx_total + eslint_problems + tsc_errors + vitest_failed))
echo "METRIC total_issues=$total"
echo "METRIC measure_s=$((end - start))"