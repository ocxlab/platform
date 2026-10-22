# Domain admission and generated-artifact review

Reviewed 22 October 2026, Europe/Berlin (UTC+02:00), against the uncommitted
working tree. This is a static principal-engineering review. The implementation
agent owned the active Docker, PostgreSQL, browser, and full-check runners; this
review did **not** run `task check`, a clean-checkout build, Docker builds, or a
shared test database.

## Scope reviewed

- installed-versus-enabled domain capability admission in the Go provider;
- OpenAPI `x-commerce-boundary` classification and request-time gating;
- bounded telemetry labels and the Food-only merchant MCP mount;
- internal payment subject/terms validation at the existing Food payment boundary;
- the stated Food-only boundary and proposed Food pricing version 2 direction;
- the change from committed generated code to ignored, reproducibly generated Go,
  TypeScript, and error-catalog artifacts.

This review does not treat the current runtime as a universal commerce platform,
independently deployable domain services, a shared payment ledger, or a real
payment adapter.

## Findings and disposition

| Severity | Finding | Disposition |
| --- | --- | --- |
| Medium | An incomplete domain annotation could have let a future Shopping or Lodging path bypass the enabled-domain admission rule. Model-bound catalog operations also needed to fail closed when no installed model was enabled. | Resolved. Every path is classified as `shared`, `model`, or an installed domain; startup rejects missing or unknown values. The registry is exact-model/version based, and disabled model routes fail before authentication, storage, or business dispatch. |
| Medium | The initial telemetry plan could have labelled the Food MCP as `shared`, weakening the bounded-domain observability claim. | Resolved. Food MCP mounting and its telemetry are Food-specific; the tests cover disabled Food and the bounded `ocp.domain` label. |
| Medium | Validating a caller-supplied payment projection without preserving the validated typed terms leaves a second, parallel representation vulnerable to future drift. | Resolved. The payment persistence boundary receives and persists validated `payment.Terms` derived from the locked Food order, rather than letting a caller select its domain or amount. |
| Medium | The first Food pricing-v2 proposal scheduled prices but did not define scheduled availability, exact temporal bounds, or concurrent retained-model behavior. | Resolved in the current design record. `docs/FOOD-PRICING-V2.md` now specifies manual-and-scheduled availability, business-local evaluation, UTC validity, bounds, quote-expiry boundaries, and concurrent V1/V2 validation/pricing. Its required verification remains future implementation work. |
| Low | The policy change to ignore all generated outputs could leave a fresh checkout without an entrypoint for generated Go, web types, or error catalogs. | Resolved by the reviewed Task and Docker changes. `task generate` is the explicit aggregate command; Go compilation paths depend on `generate:go`, web paths on `web:generate`, and runtime image builds regenerate in image stages without receiving these files from build context. |

## Static engineering assessment

The present admission seam is appropriately narrow. `food` is the only installed
domain; `none` preserves only shared account/store operations. Unknown, duplicate,
and uninstalled selections reject at startup. This avoids a misleading Shopping
or Lodging configuration switch before those domains have contracts and owned
implementations.

The no-generated-code policy is coherent in the reviewed sources:

- `.gitignore` and `.dockerignore` exclude the generated Go API, embedded error
  catalog, TypeScript API types, and TypeScript error catalog.
- `generated:check` checks Git index tracking rather than rejecting harmless local
  generated files.
- `scripts/generate-errors.mjs` creates required output directories, so a clean
  worktree does not rely on pre-existing generated paths.
- API image construction generates the error catalog and OpenAPI Go output inside
  the image. The web image bundles the contract and generates both web artifacts
  inside its build stage.
- The Task dependency graph is acyclic. Within a single Task invocation the
  dependencies serialize prerequisites sufficiently; concurrent independent Task
  invocations are not a supported isolation boundary because they intentionally
  share ignored outputs and compose volumes.

## Current limits

The review accepts neither a claim nor an implication that this is already a
multi-domain deployment architecture. The current persistence, order lifecycle,
payment table, and synchronous development adapter remain Food-specific. The
shared PostgreSQL cluster remains a capacity and failure-domain boundary. Future
selective API/worker processes need owned domain persistence, transactional
outbox/reconciliation, connection limits, and multi-process contention/recovery
evidence, as recorded in `docs/DOMAIN-BOUNDARIES.md`.

Likewise, Food pricing v2 is a design decision, not an enabled endpoint or a
completed menu editor. Retained V1 terms must never be interpreted by V2 code.

## Verification boundary and verdict

Targeted tests reported by the implementation agent are not represented here as
reviewer-executed evidence. The full `task check`, Docker clean-context builds,
and clean-checkout generation verification were still running at the time of this
review. They remain the required mechanical acceptance evidence.

**Static acceptance, conditional on those declared checks.** No remaining
source-level blocker was found in the bounded admission or generated-artifact
policy. A small documentation follow-up is still advisable: `docs/STATUS.md`
contains historical wording that generated code was committed; preserve history if
needed, but add a current-policy note so a new contributor does not infer that
committing generated outputs remains permitted.
