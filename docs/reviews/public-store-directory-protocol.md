# Public store directory protocol review

**Review date:** 21 October 2026, Europe/Berlin (UTC+02:00)

**Snapshot:** uncommitted change relative to `0f24ce5`, reviewed statically.
**Scope:** the local anonymous store directory only. This review did not run the
generation, Docker, PostgreSQL, or browser checks already in progress.

## Sound boundaries

- `GET /api/v1/public/stores` is explicitly anonymous (`security: []`,
  `x-access: public`) and is added to the server's allowlist. It neither weakens
  merchant/provider authentication nor presents itself as ONIX or network-wide
  discovery.
- The security-definer query exposes a deliberately small, read-only projection:
  merchant ID, store ID, and display name. It requires a published public
  document and excludes suspended stores. The integration coverage also checks
  that the application role cannot use the directory as general cross-tenant
  store access.
- Search is bounded and literal: `position` does not make `%` or `_` into
  wildcards, and the contract caps `q`. Result pages are bounded at 100, ordered
  deterministically by merchant/store ID, and correctly documented as live rather
  than a snapshot. The homepage uses the same public contract rather than a
  UI-only database query.

## Findings

| Priority | Evidence | Finding and required correction |
| --- | --- | --- |
| Medium | `contracts/openapi/v1/paths/public-stores.yaml`; `internal/persistence/public_stores.go`; `00005_public_store_directory.sql` | The continuation token contains only the global `(merchant_id, store_id)` position and is accepted with any `q`. A caller can therefore take a cursor obtained for one query and apply it to another; results for the latter search silently omit its matching entries before that position. Bind the cursor to a canonical query (for example, encode and validate a query digest in an otherwise opaque cursor), or reject a cursor whose query differs. The homepage retains `q` correctly, but external API callers must receive correct paging semantics too. |
| Release gate, not this slice's blocker | `paths/public-stores.yaml`; `public_stores.go`; `00005_public_store_directory.sql` | This is another unauthenticated, potentially scan-heavy public read path. The fixed result size controls response size, not query work or request volume. Record it with the existing public-abuse/release controls: apply an edge/application rate limit and revisit search indexing/caching as the local directory grows. This is not a request for ONIX or federated discovery in the current batch. |

## Bounded verdict

The public projection, publication/suspension visibility semantics, local-only
scope, and API/UI separation are appropriate. Correct filter-bound continuation
semantics before accepting the API contract. Subject to that correction and the
already-running checks, this is a sound local storefront-directory slice; it is
not a claim of network-wide discovery or a substitute for later federation.

## Final static disposition

**Snapshot:** follow-up static review on 21 October 2026 after the cursor
correction. Runtime verification remains owned by the implementation run.

**Resolved:** the continuation now prefixes a SHA-256 digest of the canonical
trimmed query and rejects a digest mismatch before querying. It retains the
deterministic UUID tuple only as the position inside that same filtered result
set. The OpenAPI description names the mismatch as a `400`, the UI begins a new
search without `after`, and the integration scenario exercises reuse with a
different query. This supplies correct paging semantics without turning the
cursor into an authorization credential.

**Protocol disposition: accepted, contingent only on the already-running
generation, integration, and browser checks passing.** The anonymous abuse and
search-performance observation remains a public-release gate, not a reason to
claim current ONIX/federated discovery or to expand this bounded directory slice.
