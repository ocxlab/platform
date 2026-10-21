# Step 1 engineering review

Reviewed 21 October 2026, Europe/Berlin. Snapshot: `docs/FOUNDATION-DECISIONS.md`,
`docs/IMPLEMENTATION-KICKOFF.md`, and `AGENTS.md`. This review assesses the
foundation handoff before implementation; it is not an implementation, security
assessment, or production approval.

## Bounded verdict

**Proceed to step 2 after recording the required implementation constraints below.**
The selected boundary is appropriately small: JSON over versioned OpenAPI, modular
source documents, an owned Go runtime, PostgreSQL, explicit migrations, and a
single managed operator. It preserves the later ONIX/UCP integration boundaries
without pretending that either is already running. The planned first slice is
also correctly limited to merchant-authorized store and draft-offering work.

The review does not recommend a broker, microservice split, public identity
provider, native ONIX runtime, payment adapter, or browser/MCP implementation
before this slice. Those would enlarge the foundation without proving its core
authorization and persistence properties.

## Required before or during step 2

| Severity | Finding | Required change or acceptance proof |
| --- | --- | --- |
| High | Forced row-level security is not sufficient if the API connects as a table owner, superuser, or role with `BYPASSRLS`. PostgreSQL role attributes and ownership determine the real boundary. | Create a distinct non-owner, non-superuser, non-`BYPASSRLS` runtime role. Keep migration/bootstrap ownership separate. Enable and force RLS on every tenant table. Add integration tests that demonstrate wrong-merchant denial using the runtime role, and that a pooled connection cannot retain a previous transaction's merchant setting. |
| High | A “typed request fingerprint” needs an explicit canonicalization rule. Hashing raw JSON makes field order and harmless representation differences produce accidental replay conflicts; hashing only selected fields risks treating distinct mutations as the same request. | Decode and validate into the versioned request type, apply documented defaults, then compute a deterministic canonical representation of the complete mutation input. Scope the idempotency key by merchant, authenticated principal/grant, and operation/route. Persist the fingerprint, completed response, status, and creation/expiry metadata in the same transaction as the mutation. Test reordered JSON, duplicate key with changed body, and cross-operation/key-scope attempts. |
| Medium | The document says generated bundles are build artifacts, but does not yet decide whether generated Go code is committed or generated in the container immediately before compile. Either can work; a mixture creates drift. | Choose one reproducible rule before adding generated code. Recommended for this repository: commit generated Go artifacts for reviewable diffs, regenerate only through `task`, and have `task check` fail when regeneration changes tracked files. Keep bundled OpenAPI output ephemeral unless a consumer needs a published bundle. Never edit generated files. |
| Medium | `OpenAPI 3.0.3` with local modular YAML is feasible, but tool compatibility and reference behavior must be tested on the exact source layout before business handlers are written. | Put a tiny valid contract through validation, bundling, generation, compilation, and a generated-client round trip. Use repository-relative references only; reject unresolved references and duplicate component names. Pin tool modules/checksums in the Go module or a tools module, rather than relying solely on prose version numbers. |
| Medium | Opaque bearer grants are acceptable for a founder-operated development slice, but a raw token must never be recoverable from database records or ordinary logs. SHA-256 is a reasonable verifier only if tokens have high entropy. | Generate at least 256 bits with a cryptographic RNG, display plaintext only once in the development provisioning command, store only its digest, use constant-time digest comparison where applicable, redact `Authorization`/credentials, and define revocation/expiry checks before action execution. Return indistinguishable authentication failures where the distinction would disclose a valid grant. |
| Medium | “Versioned draft offerings” and a replaceable Food model can accidentally become a generic JSON blob with no migration or validation policy. | Persist `model_identifier`, `model_version`, and validated model document separately from draft revision metadata. Declare a small handler/validator registry boundary. Reject unknown model versions explicitly. Treat immutable revisions as append-only; use optimistic concurrency only for selecting the current draft head. Do not create inventory allocation tables in this slice. |
| Medium | The first store API needs a durable merchant/store relationship without prematurely deciding public discovery identity or domains. | Keep the internal merchant and store identifiers stable, tenant-scoped, and separate from display name and hostname. Model endpoint/domain fields only as unset or development values unless the slice actually validates them. Do not make a custom hostname a tenancy key. |
| Low | Goose embedded migrations are a sensible explicit migration mechanism, but applying them from the API image can blur runtime and migration privileges. | Build a dedicated migration command/image target which receives the migration credential only when invoked. Keep API startup free of DDL. Exercise a second migration run as a no-op in PostgreSQL integration tests. |
| Low | The acceptance list correctly distinguishes menu-definition validation from customer selection validation, but contract examples can blur the line. | Label examples and endpoint descriptions as merchant catalog administration. Do not expose purchase, quote, checkout, payment, publication, or ONIX paths merely as empty generated stubs. |

## Confirmed design choices

- JSON is the appropriate initial payload format. It aligns with OpenAPI tooling,
  browser/API clients, and later MCP adapters. There is no demonstrated need for
  a binary alternative in this administrative slice.
- Separating reusable request/response/schema/parameter material from paths is
  maintainable when ownership is visible and a single entry document assembles
  the public API. Avoid one shared “common” file that becomes an unowned dump.
- The first Food menu model should be a named offering-model implementation, not
  the system's generic commerce object. Manual item availability belongs in its
  own small operational field or action and does not require stock accounting.
- `/api` and `/mcp` are addressing conventions to preserve, not endpoints to
  implement now. The merchant/store hostname remains separate from the provider
  API's local development endpoint.
- Merchant pickup and own delivery are valid initial fulfillment modes. External
  fulfillment remains a later separately authorized contract, so it should not
  appear in the first database schema as a speculative task engine.

## Suggested contract source layout

The following is a maintainable starting point, not a requirement for one file per
schema:

```text
contracts/openapi/
  provider.v1.yaml                 # entry document: info, servers, assembled paths
  paths/
    merchant-stores.yaml
    offering-revisions.yaml
    operational-health.yaml
  components/
    schemas/
      identifiers.yaml
      errors.yaml
      merchants.yaml
      stores.yaml
      offerings.yaml
      food-menu-v1.yaml
    parameters/
      merchant-id.yaml
      replay-key.yaml
      revision-precondition.yaml
    responses/
      errors.yaml
    security/
      bearer-grant.yaml
  examples/
    food-menu-v1.yaml
```

Keep a component near the domain that owns its meaning. For example, `food-menu-v1`
may reuse identifier and error primitives but must not turn its item/variant
semantics into shared generic fields. Contract versioning belongs in the entry
document and media/API policy, while offering model versions belong in the
offering record and payload.

## Deferred deliberately

No finding requires selecting a production identity provider, cloud vendor,
message broker, multicurrency payment model, external fulfillment operator,
custom-domain automation, or a public registration flow. Those are not missing
from the foundation; they are outside its declared acceptance boundary.
