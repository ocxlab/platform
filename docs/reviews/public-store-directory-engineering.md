# Public store directory engineering review

Static review on 21 October 2026, Europe/Berlin (UTC+02:00), of the uncommitted
directory change relative to `0f24ce5`. Shared Docker checks were already
running under the implementation owner and were not run again here.

## Finding

| Severity | Finding | Location | Required bounded correction |
| --- | --- | --- |
| Medium | `commerce.list_public_stores` is the new intentional cross-tenant read boundary, but its SECURITY DEFINER ownership, fixed search path, and ACL are not yet regression-tested like the credential lookup functions. A future migration that changes any one of those properties could turn the public endpoint into either an unsafe broad escape hatch or a runtime failure. | `services/provider/internal/persistence/migrations/00005_public_store_directory.sql`; `services/provider/internal/integration/public_stores_test.go` | Add one integration assertion against `pg_proc` for this exact regprocedure: `prosecdef`, owner different from `ocp_app`, fixed `search_path=pg_catalog`, `ocp_app` EXECUTE, and no PUBLIC EXECUTE. Keep the existing direct RLS-denial assertion. |

## Reviewed boundaries

- The function returns only `merchant_id`, `store id`, and `display_name`, and
  filters to a non-suspended store with a retained public projection. It does
  not expose raw Offering documents, draft revisions, credentials, order data,
  or merchant-private metadata.
- The public API is explicitly anonymous in OpenAPI and server startup checks.
  It relies on a fixed internal API origin from SSR; no merchant login, bearer
  credential, browser storage token, ONIX runtime, or external discovery
  dependency is introduced.
- Literal case-insensitive name matching is parameterized through SQL rather
  than assembled as SQL. `%` is correctly not treated as a wildcard. The
  continuation value is positional, query retention is documented, and the UI
  preserves it with the search query.
- The homepage renders the returned names as text and constructs same-origin
  internal links from validated UUID fields. Errors are mapped through the
  existing localized safe-message path rather than exposing upstream details.

## Verdict

**Conditional acceptance after the narrow function-hardening regression test
and the owner’s pending generation, formatting, Task, and browser checks.** No
broader discovery, search index, login, ONIX, or federation requirement is
needed for this local guest-testing directory.

## Final static disposition — 21 October 2026

The required regression guard is now present in
`services/provider/internal/integration/public_stores_test.go`. It checks the
exact `commerce.list_public_stores(text,uuid,uuid,integer)` procedure for
SECURITY DEFINER, a non-runtime owner, the fixed `search_path=pg_catalog`,
runtime-only execute authority, and no PUBLIC execute grant. The existing test
continues to prove that direct cross-tenant `commerce.stores` access remains
subject to RLS.

The cursor is also now bound to the normalized search query by its SHA-256
digest, with contract and integration coverage for mismatched-query rejection.
That prevents accidentally applying a continuation position from one result set
to another.

**Static acceptance, pending the owner’s active generation, formatting, full
Task, and browser verification.** No remaining source-level engineering or
security blocker is identified for this deliberately local public directory.
