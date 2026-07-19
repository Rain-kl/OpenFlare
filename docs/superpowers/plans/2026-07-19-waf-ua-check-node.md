# WAF UA Check Node Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add WAF graph node `ua_check` (require UA, browser/OS whitelist with and/or, bot/abnormal blocks) end-to-end: validate/compile, Lua runtime, editor UI.

**Architecture:** Match-node pattern like `geo_match`. Control plane stores `UACheckConfig`; edge classifies `http_user_agent` with analytics-equivalent token rules; evaluation order: require → block bots → block abnormal → whitelist.

**Tech Stack:** Go (waf package), Lua (OpenResty waf_runtime), React/TS editor, Vitest, Go tests.

**Spec:** `docs/superpowers/specs/2026-07-19-waf-ua-check-node-design.md`

## Global Constraints

- Type `ua_check`; handles `true`/`false`.
- Config fields: `require_ua`, `browsers`, `operating_systems`, `match_mode` (`and`|`or`, default `or`), `block_common_bots`, `block_abnormal_ua`.
- Closed enums for browser/OS labels matching analytics.
- Block before whitelist; empty lists = no whitelist constraint.
- No schema_version bump; no new HTTP API.
- Changelog + Chinese design doc update.

## File Map

| File | Role |
|------|------|
| `internal/apps/openflare/waf/graph_types.go` | Type + config |
| `internal/apps/openflare/waf/graph_validate.go` | Validate + handles |
| `internal/apps/openflare/waf/graph_compile.go` | Compile normalize |
| `internal/apps/openflare/waf/*_test.go` | Go tests |
| `internal/apps/agent/nginx/waf_runtime.lua` | Runtime eval |
| `internal/apps/agent/nginx/waf_runtime_spec.lua` | Lua specs |
| `internal/apps/agent/nginx/manager_test.go` | Embed smoke if needed |
| Frontend editor components + types | UI |
| `docs/design/waf-orchestration-design.md` | Node table |
| `docs/changelog/index.md` | Unreleased |

### Task 1: Backend types/validate/compile
### Task 2: Lua runtime + specs
### Task 3: Frontend editor
### Task 4: Docs + gates

(Detailed code follows during implementation; execute TDD per layer.)
