# 2026-11-25: Repository Layout Restructure (Phases 1–4)

## Context & Decisions

1. **Keep the Go module at the repository root**:
   - `go.mod` / `go.sum` stay at the root so `go test ./...`, CodeQL autobuild, and tools importing `internal/` keep working without module-boundary workarounds.
   - The module name remains `github.com/Rain-kl/Wavelet`; renaming it was deliberately deferred to a separate mechanical change.

2. **Phase 1 — deployment fixes** (`9d7ea498`):
   - Removed the stale `docker/Dockerfile.agent` reference from `docker-compose.yaml` and replaced hardcoded agent/discovery/tunnel tokens with `${...}` environment variables.
   - Dropped `COPY docs ./docs` from `docker/Dockerfile` and `docker/Dockerfile.backend`; the backend embeds Swagger at compile time and never reads the docs site at runtime.
   - Deleted the unreferenced `scripts/agent-entrypoint.sh` and `scripts/tidy.sh`, removed `.auto/` per request, and fixed stale `EXTRA_FILES` entries in `build-release.yml`.

3. **Phase 2 — Swagger, tools, and docs cleanup** (`e292392a`):
   - Moved generated Swagger output from `docs/` to `api/swagger/` and updated the route import, generator scripts, and lint exclusions.
   - Moved internal development scripts from `scripts/` to `tools/{codegen,git,lint,smoke}/`; only user-facing install/update/uninstall scripts remain public under `scripts/`.
   - Moved `scripts/edge-entrypoint.sh` to `docker/entrypoints/`, `docs/OpenFlare-Brand/` to `docs/assets/brand-kit/`, and `docs/superpowers/` to `.agents/notes/superpowers/`.
   - Removed the `.auto/` special case from `audit-non-english.mjs` after its deletion.

4. **Phase 3 — shared sources** (`7d5bf8a6`):
   - Added `shared/edgeassets/` with `assets.go` (`//go:embed`) as the single source for the brand mark, PoW templates, and site-auth page. Go imports it directly; Rust Edge and the frontend generator read the same files without a copy/sync step.
   - Moved `openflare-probe` from `probe/` to `shared/probe/` and updated the Edge path dependency, Docker build context, `.dockerignore`, and Edge CI path filters.
   - Docker now copies only `edge/`, `shared/probe/`, and `shared/edgeassets/` into the Edge builder.

5. **Phase 4 — flatten Edge** (this commit):
   - Promoted `edge/pingora/*` to `edge/*`; the redundant nesting was a historical leftover from the OpenResty migration.
   - Updated `Makefile`, `Dockerfile.edge`, `.dockerignore`, all three Edge workflows (including the canary diff filter and OWASP CRS license path), and the Chinese/English design and CLI docs.
   - The Rust include paths now resolve as `edge/src/policy/* → ../../../shared/edgeassets/*`, and the probe dependency is `edge → ../shared/probe`.

## Invariants Preserved

- Public URLs and release contracts remain unchanged: `scripts/{install,update,uninstall}-*.sh`, `scripts/fetch-agent-geoip-mmdb.sh`, `config/clickhouse/performance.xml`, `config.example.yaml`, `.env.example`, and existing `docs/` routes.
- `shared/edgeassets` is a normal package inside the root Go module, so `//go:embed` works without generated copies or a second Go module.
- Edge and probe retain independent Cargo workspaces with locked dependencies.

## Verification

- `make swagger`, `make license-check`, attack-rule generator check, and automation-catalog check pass.
- `go build ./...` plus `go test ./internal/router/... ./internal/automationcontract/... ./internal/openflare/...` pass.
- `cargo test --manifest-path edge/Cargo.toml --locked` (48 tests) and `python3 edge/tests/runtime_smoke.py edge/target/debug/openflare-edge` pass.
- Frontend `generate-pow-assets --check`, i18n checks, `tsc`, and ESLint pass; `docs/` Fumadocs build passes.
- `make code-check` still stops at a pre-existing golangci-lint backlog (83 findings across tunnel, agent, logstore, and scheduler files untouched by this restructure); no finding points at the moved or new code.
