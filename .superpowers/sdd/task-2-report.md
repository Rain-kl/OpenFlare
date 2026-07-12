# Task 2 report — Zone domain and API

## Delivered

- Added the `internal/apps/openflare/zone` domain layer. Zone roots and explicit hostnames are normalized with `publicsuffix.EffectiveTLDPlusOne`; root creation requires exact eTLD+1 equality and hostnames must belong to the selected Zone.
- Wildcards, empty values, protocols, paths, query/fragment/userinfo forms are rejected. A supplied certificate is checked before persistence.
- Added `migrate-zones`, an explicit transactional and idempotent importer. It reads route domains through `routeidentity.DecodeDomains`, aligns legacy `domain_cert_ids` by index, only uses `of_managed_domains` when no route domains exist, and rolls back with all discovered conflicts.
- Registered authenticated `/api/v1/d/zones` list/create, `/:id/overview`, and `/:id/domains` create endpoints. Retired the `managed-domains` route block and its Swagger paths, while preserving the legacy model/table and logic for migration compatibility.
- Added focused TDD tests for wildcard rejection and PSL handling, plus authenticated integration coverage for Zone/domain creation and the 400 response envelope.

## Verification

- `go test ./internal/apps/openflare/zone ./internal/apps/openflare/integration -count=1`
- `make code-check`
- `make swagger` (the generator prints pre-existing octal-constant evaluation warnings; generation succeeds)
- Confirmed generated Swagger contains `/zones` and no `/managed-domains` paths.
