# Step 1 protocol and contracts review

**Reviewer perspective:** protocol/contracts

**Reviewed:** 21 October 2026 (Europe/Berlin, UTC+02:00)
**Snapshot:** `docs/FOUNDATION-DECISIONS.md` and
`docs/IMPLEMENTATION-KICKOFF.md`; no OpenAPI source, generated code, migrations, or
business handlers exist at this review point.

## Scope and verdict

The foundation makes sound bounded choices: JSON is the appropriate first wire format;
OpenAPI 3.0.3 is a pragmatic target for the chosen Go generator; generated code is not
the source of truth; PostgreSQL-level isolation and immutable offering revisions keep
the first slice from confusing catalog editing with purchase authority. Keeping the
owned merchant administration API separate from claimed Beckn/UCP wire compatibility
is also correct. It permits selected UCP-compatible commerce semantics later without
pretending that a Food or universal-product protocol has already been adopted.

**Verdict: proceed to Step 2 after resolving the blocking contract decisions below in
the implementation handoff.** They are deliberately small decisions, but must be
written before a generated server/client or migration makes them difficult to change.
This is a review of a plan, not a conformance, security, or interoperability approval.

## Findings

| Severity | Finding | Required bounded correction |
| --- | --- | --- |
| Blocker | The public contract version boundary is not chosen. `/api` and `/mcp` describe endpoint placement, but not whether a client reaches an unversioned mutable contract or a stable version. | Make the first REST surface explicit, e.g. `/api/v1`, and give the entry document an unambiguous identity such as `contracts/openapi/v1/openapi.yaml`. State the compatibility rule: additive optional fields may be added within v1; removed/meaningfully changed behavior gets a new major path and entry document. `/mcp` remains an advertised later adapter, not a v1 REST operation. |
| Blocker | Private-operation authority cannot be generated or tested consistently yet. “Merchant and action scope” does not identify the bearer scheme, scope vocabulary, store/merchant binding, or the difference between missing/invalid and insufficient authority. | Put one `BearerAuth` security scheme and reusable scope descriptions in the root contract. Define the initial grants narrowly (for example `store:read`, `store:write`, `offering:read`, `offering:write`), bind every grant to one merchant, and derive merchant scope from the grant rather than trust a client-supplied merchant ID. Standardize `401` for absent/invalid/revoked/expired credentials and `403` for a valid grant lacking an action or tenant entitlement. |
| Blocker | “Required scoped replay key” and “complete typed request fingerprint” are valuable but not an interoperable rule. Without a header name, replay scope, response retention, and mismatch behavior, clients and database uniqueness constraints will diverge. | Specify one reusable request header, preferably `Idempotency-Key`; require it on mutating operations. Scope uniqueness at least by authenticated grant/merchant plus operation, store the canonical semantic request fingerprint and final response atomically, return the original status/body for a matching replay, and return `409` for same key with a different request. Define a documented retention window before implementation. |
| High | Optimistic offering revision is not an API concurrency contract. A generated type alone cannot tell a caller how to avoid silently overwriting a newer draft. | Choose and document one mechanism for v1: an `If-Match` entity tag on update, or a required `baseRevision` field. For the immutable revision model, `baseRevision` is clear: create a new draft revision only if it matches the current head; return `409` with the current revision on conflict. Do not support both mechanisms initially. |
| High | The desired maintainable source layout is described only conceptually. It needs a concrete root/paths rule before references and generated files appear. | Create a contract layout with a single entry document, `paths/`, `schemas/` divided by bounded domain (`common`, `identity`, `store`, `offering`), and `components/` for reusable parameters/responses/security. Keep behavioral endpoint declarations in `paths/`; use schemas for data shape only; keep examples near the operation/schema they explain. Generate into a clearly ignored derived directory outside sources. |
| High | Error payloads and status semantics are not defined. This would lead each first endpoint to invent a different JSON error and makes UI, CLI, agent, and generated clients less reliable. | Define a reusable `Problem` response (`application/problem+json`, compatible with RFC 9457) plus a stable machine `code`, correlation/request identifier, and optional invalid-field entries. Reuse it for validation (`400`), authentication (`401`), authorization (`403`), missing scoped record (`404`), replay/revision conflict (`409`), and unsupported offering model (`422`). Do not expose whether another merchant owns a resource. |
| Medium | The offering boundary correctly rejects a universal product model, but the contract must make the replacement seam explicit. Otherwise a Food-menu JSON shape will become the accidental common catalog schema. | Model an immutable offering revision envelope with a required `model` identifier and `modelVersion`, and a model-specific document payload. The first `food-menu` model can have a concrete schema/validator; unknown model/version returns `422` until installed. Do not encode future lodging, appointment, or Shopping fields in this first contract. |
| Medium | The tooling cohort is partly pinned but cannot yet be reproduced exactly: `kin-openapi` and the bundling/drift command version are unnamed, and generated-output ownership is not specified. | Pin all Go module versions in `go.mod`/`go.sum`, pin any container/tool image used for generation, and add Task targets for validate, bundle, generate, and drift-check. Commit generated Go only if the chosen code-review workflow needs it; otherwise exclude it and make generation part of every build. Select one policy and test it. |
| Medium | OpenAPI 3.0.3 is suitable here, but its schema dialect should not be used to claim generic JSON Schema validation. The handoff recognizes this distinction; the contract structure must preserve it. | Keep reusable request/response schema references within the OAS document set. Put semantic model validation behind the server boundary, with named validation errors. Do not try to use `oneOf`/discriminators as a universal plugin registry or infer purchase validity from generated forms. |
| Low | Contract examples are required, but their status is not stated. Examples can accidentally become test data, a normative promise, or merchant data. | Use fictional values, label examples illustrative, and validate them in the contract test task. Add a small approved example per success and important failure case; no real merchant/product data. |

## Recommended v1 source shape

This is a maintainability recommendation, not a premature module explosion:

```text
contracts/
  openapi/
    v1/
      openapi.yaml                 # info, servers, tags, security, root paths
      paths/
        stores.yaml
        offerings.yaml
      schemas/
        common/problem.yaml
        identity/merchant.yaml
        stores/store.yaml
        offerings/offering-revision.yaml
      domains/
        food/menu/v1/schema.yaml   # model-specific document shape
      components/
        parameters/idempotency-key.yaml
        responses/problem.yaml
        security/bearer-auth.yaml
      examples/
        ...
  generated/                       # derived; commit policy stated and enforced
```

The exact filenames can differ, but each referenced file should have one clear
semantic role. The explicit `model.id`/`model.version` envelope, bounded document
object, separately reusable Food menu schema, and registered server validator are the
right extension seam: they reject unsupported models rather than executing arbitrary
schemas or inferring arbitrary handler behavior. Reuse should happen through small components—not a single all-purpose
`resources.yaml`, and not copy-pasted fields across operations. A root OpenAPI document
may reference only local, reviewed files in this slice; remote `$ref` values make
generation and supply-chain review less predictable.

## Contract behavior that Step 2 should prove

- A merchant-scoped bearer grant can create and read only its own store and offerings.
- Every mutation requires and records an idempotency key; a same-key/same-request retry
  returns the original result, while a same-key/different-request retry is rejected.
- A competing edit to the same draft produces the documented revision conflict rather
  than overwriting another editor.
- The service validates the `food-menu` model structurally and semantically, but makes
  no purchase, inventory, allocation, publication, ONIX, payment, or external
  fulfilment claim.
- Generated server/client types compile from the bundled v1 contract; validation,
  generation, and drift checks run in Docker through Task.

## Boundaries retained

- JSON over HTTPS is the v1 REST payload decision; it does not preclude future envelopes
  or transports.
- `/api` and `/mcp` remain hostname-relative addressing conventions. Custom-domain
  onboarding and MCP routing are outside this slice.
- Manual restaurant availability is offering configuration, not inventory accounting.
  Own delivery/pickup are merchant fulfilment; external fulfilment requires a future
  explicit contract.
- Native ONIX publication/crawl remains a separately retained runtime boundary, and
  selected UCP semantics may be evaluated at the commerce boundary later. Neither is
  implemented or claimed by this administration API.
