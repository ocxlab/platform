# Step 2 protocol and contracts review

**Reviewer perspective:** protocol/contracts

**Reviewed:** 21 October 2026 (Europe/Berlin, UTC+02:00)

**Snapshot:** uncommitted Step 2 working tree, including modular OpenAPI v1 source,
generated Go artifact, HTTP/persistence implementation, and PostgreSQL integration
tests. `task check` was running under the implementation owner and is not claimed as
executed by this review.

## Verdict

The implementation realizes the important Step 1 choices rather than merely
documenting them. The source contract is modular and locally owned; JSON is the
appropriate payload format; `/api/v1` has an explicit compatibility statement;
the Food model remains a separately installed, bounded model; and the API does not
claim Beckn/UCP conformance. The implementation also has meaningful evidence for
grant-scoped authorization, immutable revisions, canonical replay, SQL transaction
atomicity, forced RLS, and concurrent update behavior.

**Verdict: substantial foundation, but correct the three blockers before calling
Step 2 accepted.** The remaining findings are small enough to address in this batch;
none asks for ordering, federation, payment, or a universal catalog model.

## Findings

| Severity | Evidence | Finding and bounded correction |
| --- | --- | --- |
| Blocker | `contracts/openapi/v1/domains/food/menu/v1/schema.yaml:60-92`; `services/provider/internal/offering/food/menu.go:26-34` | An option group and an option have identifiers and selection/price data but no customer-visible name. A merchant can therefore store an opaque `extra-cheese` ID but cannot describe the group or choice to an application. This fails the intended bounded Food model for variants and extras. Add required, length-bounded `name` fields to `OptionGroup` and `Option`, and reject blank names in the semantic validator. This remains Food-specific and does not make a universal product schema. |
| Blocker | `contracts/openapi/v1/components/responses/problem.yaml:2-6`; `services/provider/internal/httpapi/server.go:81-88`; integration test only expects `400` for an oversized body at `internal/integration/foundation_test.go:200-206` | The contract says `413` for a body that is too large and `415` for an unsupported media type. Middleware maps every OpenAPI/body-validation failure to `400`, including the `MaxBytesReader` limit. Either distinguish these cases and emit the advertised statuses, or remove those status claims from the reusable Problem response description. Prefer the former, and add response-validation tests for both statuses. |
| Blocker | Step 1 review required a correlation/request identifier; `contracts/openapi/v1/schemas/common/problem.yaml:1-21` has none and `services/provider/internal/httpapi/server.go:109-120` emits none | Clients and support staff cannot correlate a returned problem with an API request. Add a generated-or-accepted request identifier to every response (for example `X-Request-Id`) and include it as `instance` or an explicit field in Problem Details. Define its syntax in a reusable header component and test both successful and failed operations. Avoid logging credentials or request bodies. |
| High | `contracts/openapi/v1/openapi.yaml:33-49`; generated identifiers at `services/provider/internal/httpapi/generated.go:111-142, 236-257` | The source is sensibly split, but external-reference-derived generated public Go names such as `DomainsFoodMenuV1SchemaYamlOptionGroup8c47bbaf` and `ComponentsParametersMerchantIdYamldbf0a53e` make the committed client/API uncomfortable to maintain. Materialize stable, intentional root component aliases for reusable parameters, responses, and domain schemas, or use supported generator naming configuration. Regenerate and retain a test that exposes the generated client. Do not hand-edit generated code. |
| High | `contracts/openapi/v1/components/responses/problem.yaml:1-19`; all path files use only `default` error responses | One generic `default` response correctly keeps the source compact, but it hides each operation's expected security/validation/conflict behavior from generated clients and rendered documentation. Declare the actual reusable named responses for at least `400`, `401`, `403`, `404`, `409`, and `422` on each applicable operation, while retaining a generic fallback only for unexpected `5xx`. This does not require distinct JSON shapes. |
| Medium | `services/provider/internal/httpapi/server.go:44-45, 75-77`; `contracts/openapi/v1/components/security/bearer.yaml:5-8` | Action policy is duplicated in a Go operation-ID map and in prose. A new protected operation is safely denied because its missing map entry maps to an empty scope, but the drift is silent and the contract is not the reusable authority catalog. Define an explicit operation-to-scope declaration close to the contract metadata or a checked application registry, and add a test that every protected OpenAPI operation has exactly one known action scope. |
| Medium | `contracts/openapi/v1/paths/store.yaml:9-16`; `paths/offering.yaml:9-17`; `paths/offering-revision.yaml:16-24`; `paths/health.yaml:7-14` | Read/liveness success descriptions say “matching retries return this original status and body.” Idempotency is not applicable to `GET`; the statement obscures the mutation rule. Keep the wording only on `POST`/`PUT`, and use ordinary successful-read/liveness descriptions elsewhere. |
| Medium | `services/provider/internal/httpapi/server.go:122-143`; `contracts/openapi/v1/schemas/common/problem.yaml:1-21` | The error type is structurally checked by the response tests, but `Problem.type` is always `about:blank` and error details have no declared field-level issue format. `about:blank` is valid as a generic RFC 9457 type, but it yields little client guidance. Keep the stable `code`, but publish stable provider documentation URIs for distinct problem classes when available, or state that `code` is the sole programmatic discriminator for v1. Add an optional bounded invalid-parameter array only if UI/client validation needs it; do not invent one prematurely. |
| Low | `contracts/openapi/v1/components/parameters/idempotency-key.yaml:9-10`; `services/provider/internal/persistence/migrations/00001_foundation.sql:53-65` | The foundation intentionally retains successful keys indefinitely. That is coherent for a tiny launch slice, but the absence of an expiry/archival migration plan needs an operational bound before merchants generate years of draft writes. Record the retention review trigger (for example, before general availability or a measured volume threshold). Do not implement cleanup now, because reuse semantics must be contracted first. |

## Confirmed behavior

The following choices are correctly reflected in code and tests:

- The public source entry is modular, validates only local references, bundles into a
  generated artifact, and then reloads/revalidates that artifact
  (`cmd/contracts/main.go:29-123`). Generated Go is derived and drift-checked through
  Task rather than hand-maintained.
- The public REST surface is explicitly `/api/v1`, while `info.version: 0.1.0` is
  correctly identified as the current contract-document version
  (`contracts/openapi/v1/openapi.yaml:1-28`, `contracts/README.md:57-63`).
- Liveness deliberately opts out of global bearer security
  (`paths/health.yaml:1-5`), while private routes authenticate, parse the requested
  merchant, and enforce both merchant binding and action scope before dispatch
  (`internal/httpapi/server.go:55-80`). Missing/invalid/revoked/expired grants return
  `401`; an authenticated but unauthorized action/merchant returns `403`.
- Mutations require `Idempotency-Key`; its canonical decoded-JSON fingerprint and
  stored response are committed in the same transaction
  (`components/parameters/idempotency-key.yaml:1-15`,
  `internal/persistence/resources.go:44-91`). Concurrent same-key tests demonstrate
  one mutation and byte-identical replay; revocation is rechecked before replay.
- Immutable offering revisions use explicit `base_revision`, a row lock, and a
  `409` with `current_revision` rather than last-writer-wins
  (`schemas/offerings/revision.yaml:14-32`,
  `internal/persistence/resources.go:118-137`,
  `internal/integration/foundation_test.go:238-275`).
- An arbitrary model document is not executed. The registry accepts only an installed
  `(model.id, model.version)` handler, and unknown models produce `422`
  (`internal/offering/models.go:9-33`, `internal/httpapi/server.go:178-187`).
  The first Food model remains draft-only and explicitly does not assert inventory,
  publication, purchase, or payment readiness.

## Step 2 acceptance after fixes

Accept this step when the three blockers are corrected and the integration suite adds
coverage for customer-visible extras, `413`, `415`, and request identifiers. The
existing checks should continue to demonstrate OpenAPI validation, generated-client
round trip, grant/action/tenant boundaries, canonical replay, concurrent revisions,
atomic rollback, RLS/pool cleanup, and restart persistence. This remains a foundation
acceptance, not an ordering, network, payment, production-authentication, or independent
interoperability acceptance.

## Verification and disposition — 21 October 2026

The corrected working-tree snapshot was statically re-reviewed after the implementation
owner reported a passing `task check`. This review did not concurrently run the
PostgreSQL integration suite.

| Earlier finding | Disposition | Verification |
| --- | --- | --- |
| Customer-visible Food extras | **Resolved** | `OptionGroup` and `Option` now require a bounded `name` (`domains/food/menu/v1/schema.yaml:60-100`); the semantic validator rejects blank names (`internal/offering/food/menu.go:65-76`); fixture tests cover both blank cases (`internal/httpapi/contract_test.go:55-85`). |
| Advertised `413` and `415` behavior | **Resolved** | The HTTP boundary detects `MaxBytesError` before schema validation and emits `413`; it parses and requires `application/json` for mutations and emits `415` otherwise (`internal/httpapi/server.go:97-123`). Integration tests exercise both outcomes (`internal/integration/foundation_test.go:207-230`). |
| Request correlation | **Partly resolved; one correction remains** | Runtime creates a fresh UUID response header and emits matching `Problem.instance` URNs (`internal/httpapi/server.go:60-62,143-154`); the shared Problem response declares it and integration tests verify it (`components/responses/problem.yaml:7-17`, `internal/integration/foundation_test.go:115-143`). However, successful `200`/`201` OpenAPI responses do not declare `X-Request-ID`, although the contract prose promises it for every attempt (`openapi.yaml:10-11`). |
| Stable generated names | **Resolved** | Intentional root aliases yield `MerchantID`, `FoodOption`, and related usable generated types (`openapi.yaml:31-76`, `internal/httpapi/generated.go:111-159,241-262`). |
| Explicit expected failures | **Resolved** | Operations now list their expected Problem statuses rather than exposing only a `default` response, for example create-store (`paths/stores.yaml:17-39`) and offering write (`paths/offering.yaml:47-73`). |
| Scope-policy drift | **Resolved for this slice** | Protected operations declare `x-required-scope`; startup rejects an unknown/missing scope (`internal/httpapi/server.go:46-59`) and the contract test checks every protected operation (`internal/httpapi/contract_test.go:90-107`). |

### Final verification — 21 October 2026

The remaining request-identifier correction is **resolved**. Every declared successful
response now references the reusable `X-Request-ID` header: liveness
(`paths/health.yaml:7-15`), store creation/read (`paths/stores.yaml:18-26`,
`paths/store.yaml:11-19`), and offering read/write/revision
(`paths/offering.yaml:11-19,51-59`, `paths/offering-revision.yaml:18-26`). The
contract test traverses every declared `2xx` response and fails if its header is missing
or not UUID-formatted (`internal/httpapi/contract_test.go:97-115`). Runtime behavior
continues to create a fresh header and matching Problem Details instance.

**Final protocol verdict: accepted for the bounded Step 2 foundation, subject to the
reported passing generation, formatting, contract, race, and integration checks.**
There are no remaining protocol blockers. The established scope, replay, immutable
revision, installed-model, tenant, and no-conformance boundaries remain appropriately
limited to Step 2; this is still not acceptance of ordering, federation, payment,
production authentication, or independent interoperability.
