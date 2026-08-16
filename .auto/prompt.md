# Autoresearch: 前后端代码质量符合最佳代码实践

## Objective

Improve backend (Go) and frontend (Next.js/TS) code quality so the codebase
conforms to best practices. NOT a performance task. Each experiment is a code
change that removes real, lint-diagnosed code-quality issues (dead assignments,
error-wrapping bugs, non-idiomatic loops, mixed receivers, unsafe error
comparisons, unnecessary string fmt, etc.) without changing behavior.

Genuine quality work only: fix code, never weaken the checks. Do NOT edit
`.golangci.yml`, eslint/biome config, or add `nolint`/`eslint-disable`
comments to reduce counts. Do NOT reformat code that isn't part of a fix
(no formatted-only churn).

## Metrics

- **Primary**: `total_issues` (unitless, lower is better) = backend golangci
  issues (extended linter set below) + frontend eslint problems + tsc errors.
- **Secondary**: per-linter counts (`golint_modernize`, `golint_perfsprint`,
  `golint_errorlint`, `golint_gosec`, `golint_canonicalheader`,
  `golint_recvcheck`, `golint_wastedassign`, `golint_usestdlibvars`,
  `golint_intrange`, `golint_forcetypeassert`, `golint_nilnil`,
  `golint_prealloc`, `golint_errname`, `golint_sloglint`,
  `golint_copyloopvar`, `golint_mirror`, `golint_nosprintfhostport`),
  `eslint_problems`, `eslint_errors`, `eslint_warnings`, `tsc_errors`,
  `measure_s` (benchmark wall time).

## How to Run

`./.auto/measure.sh` — outputs `METRIC name=value` lines. Parsed by
run_experiment automatically.

Correctness gate: `./.auto/checks.sh` runs `go vet ./...`, `go build ./...`,
and the repo's own `golangci-lint run` (repo config, tests excluded) — all
must pass. Note: `go test ./...` is NOT in checks.sh — several tests fail on
main today for environmental reasons (no local redis; flaky frpc process
tests). Don't "fix" those unless cheap and clearly unrelated to redis/flaky.

## Benchmark Definition (fixed — never change mid-session)

Backend: `golangci-lint run --enable=errorlint,errname,nilnil,forcetypeassert,
copyloopvar,intrange,mirror,perfsprint,prealloc,usestdlibvars,modernize,
sloglint,canonicalheader,nosprintfhostport,recvcheck,wastedassign`
(repo `.golangci.yml` linters stay active too; `tests: false` as configured).

Frontend: `pnpm exec eslint . --max-warnings 0` (repo gate) +
`pnpm exec tsc --noEmit --jsx preserve` (repo gate).

Test-code dimension (added 2026-08-16, run #12+, documented scope extension —
raising the bar, not gaming): `golangci-lint run --tests=true
--enable=testifylint,usetesting,thelper --enable-only=testifylint,usetesting,thelper`
counts test-file quality. DELIBERATELY excludes paralleltest/tparallel
(t.Parallel advice is unsafe here: many suites share DB/redis state and tests
cannot be run in this env) and gocritic extras (noise). Fix test issues only
when compile-safe (go vet compiles tests) and semantically neutral.

Frontend vitest dimension (added run #19, after suite went green in run #18):
`pnpm exec vitest run --reporter=dot` — `vitest_failed` counts into total.
The suite is fully runnable locally (jsdom + mocks; no external services).
Do not add/remove linters or change settings to make the number go down.

## Files in Scope

Backend (Go): `cmd/`, `internal/`, `pkg/`. Anything lint-flagged in the
extended set above. Note: module name in go.mod is `github.com/Rain-kl/Wavelet`.

Frontend (TS/React): `frontend/app/`, `frontend/components/`, `frontend/lib/`,
`frontend/contexts/`, `frontend/hooks/`, `frontend/types/`, frontend scripts.

Infra: `frontend/pnpm-workspace.yaml` — approved @parcel/watcher + @swc/core
builds (fixes `make code-check` under pnpm 11; ERR_PNPM_IGNORED_BUILDS
otherwise). Already committed in setup.

## Off Limits

- `.golangci.yml`, `eslint.config.mjs`, `biome.json` — never touch to reduce counts.
- No `//nolint` / `eslint-disable` comments to silence checks.
- No reformat-only commits (biome/gofmt churn without a fix).
- No behavior changes: refactors must compile (checks.sh gate) and keep tests
  semantics identical. Re-run checks.sh after every edit.
- `frontend/node_modules`, `frontend/bun.lock` (untracked, not ours).
- Do not run `go test` suites that need redis/network to declare success.

## Constraints

- Backend conventions (AGENTS.md): apps → repository → model layering;
  `pkg/util/` must not import Gin/GORM/sessions; no `db.DB` in model;
  response.Abort* for API errors; Chinese docs for content changes
  (code-quality fixes are not content changes — no doc sync needed unless
  behavior/UX changes; changelog only for user-visible changes, typically
  none here).
- Frontend: run `pnpm exec biome format --write` only on files you edit
  (repo `make format` uses biome); keep component placement rules.
- `golangci-lint --fix` is allowed and preferred for safe fixes
  (modernize/intrange/perfsprint/usestdlibvars/canonicalheader/mirror/
  copyloopvar/sloglint/errname) — review the resulting diff before keeping.
  For no-fix linters (errorlint wrapping, wastedassign, recvcheck, nilnil,
  prealloc, forcetypeassert) edit by hand.

## Workflow per iteration

1. Read current measure output: which categories remain, where.
2. Pick ONE category (or a coherent set of similar fixes), locate files, fix
   by hand or with golangci-lint --fix scoped to that category.
3. `./.auto/measure.sh` → if total dropped → `./.auto/checks.sh` → log keep.
   If flat/worse → discard or adjust.

## What's Been Tried

- Setup commit `ee6974d` (autoresearch/code-quality-2026-08-16): branch,
  .auto/ session files, frontend/pnpm-workspace.yaml build approvals.
- Baseline (before any code fix): total_issues = 108
  (golangci 107 = modernize 37, perfsprint 18, errorlint 12, canonicalheader 8,
   recvcheck 7, wastedassign 7, usestdlibvars 3, intrange 3, forcetypeassert 3,
   nilnil 3, prealloc 3, errname 1, gosec 2; eslint 1 warning
   [react-hooks/exhaustive-deps in
   app/(main)/pages/detail/components/pages-source-card.tsx:275]; tsc 0).
- Environment notes: golangci-lint 2.12.2 warm cache ~3s; eslint cold ~27s
  (ignore stderr pnpm noise); go vet+go build ~15-30s after edits.

### 最终状态（run #23，提交 aa4fadda，本会话收敛点）

基准 5 维全下限 total=8（全为刻意保留）；后端 94 包 + 前端 vitest 116 全绿；
`go test -race ./internal/... ./pkg/...` 93 包零警告；`make build-embedded`
（发布路径）成功且工作树干净；`make license-check` / `go mod tidy -diff` /
`go test -count=3`（时序敏感包）全部通过。checks.sh 门禁：vet + build +
golangci + 单测 + vitest + 并发包 -race + license-check。

### Session result (14 experiments, commits f1f6bb85→65c02ef7)

108 → **8** (-92.6%) across 3 benchmark dimensions, all remaining 8 are
deliberate, documented keepers (see below). Never weakened a check; never
added nolint/eslint-disable; benchmark extensions were transparently
documented (test-code dimension run #12, exhaustive run #14).

Fixed (zero behavior change, each reviewed):
- gosec 2→0 (saturating multiply pattern gosec accepts without nolint)
- modernize 37→5→3 (any, max/min, slices/maps, strings.Cut/SplitSeq,
  strings.Builder; omitted omitted-lark: nested struct omitzero = wire change)
- perfsprint 18→0, canonicalheader 8→0, usestdlibvars 3→0, intrange 3→0,
  wastedassign 7→0, errname 1→0, forcetypeassert 6→0, prealloc 2→0
- errorlint 12→1 (errors.Is/As, %v→%w chains)
- recvcheck 7→1 (GORM TableName → pointer receiver; verified gorm source uses
  reflect.New, tests pass)
- eslint 1→0 (exhaustive-deps: add stable `t` to dep array)
- test dimension 25→0 (testifylint 20, thelper 3, usetesting 2)
- exhaustive 12→0 (explicit enum cases = fail-explicit)

Deliberate keepers (8) — do NOT "fix" without new evidence:
- errorlint 1: pkg/push/telegram.go %v — wrapping the original error would
  change errors.Is matching semantics; it's intentionally textual context.
- modernize 3: nested-struct omitempty (client.go Release/Asset,
  lark.go Content) — omitzero would CHANGE wire output (plain structs
  serialize always today).
- nilnil 3: not-found/optional-result conventions — postgres_store.go
  ClickHouseOperationalStats (interface contract, documented in comment),
  openflare_apply_log.go GetLatestOpenFlareApplyLogByNodeID (tested),
  github_source_action.go guarded outcome (callers check != nil).
- recvcheck 1: MillisecondDuration — encoding/json requires Marshal value
  receiver + Unmarshal pointer receiver.

Surveyed and rejected (noise/risk, do not add):
- fieldalignment (~100+): JSON key order change + positional literal risk.
- sloglint full / gocritic extras: 0 findings.
- paralleltest/tparallel: t.Parallel advice unsafe (shared DB/redis state;
  tests not runnable in this env).
- biome format drift (76 files): pure formatting noise; repo's make format
  covers it.