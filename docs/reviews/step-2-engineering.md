# Step 2 engineering review

Reviewed 21 October 2026, Europe/Berlin. Snapshot reviewed: uncommitted Step 2
working tree, including `contracts/openapi/v1/`, `contracts/tooling/go.yaml`,
`services/provider/`, `deploy/local/`, `Taskfile.yml`, and
`docs/FOUNDATION-DECISIONS.md`.

Executed by this reviewer:

- `task contracts:check` — passed: modular references validate, a deterministic
  bundle is reloaded and validated, and regenerated Go equals the committed file.
- `task test:integration` — passed: PostgreSQL-backed foundation tests ran with
  the disposable `test-db` service and it was stopped afterward.

These results add to, but do not replace, the recorded full `task check` and
`task smoke` results. This is an engineering review, not production approval,
an independent interoperability test, or evidence of order/payment/network
readiness.

## Bounded verdict

**Substantial foundation work is present and technically coherent, but resolve
the runtime-role blocker below before accepting Step 2.** The executable slice
does meet its intended functional boundary: JSON OpenAPI sources are modular and
generated deterministically; the Go API authenticates grants before private
operations; store and immutable draft-offering writes use a real PostgreSQL
transaction; Food is a registered model rather than the universal catalog; and
the integration suite exercises actual RLS, replay, revision, and recovery-like
restart conditions.

The review does not recommend adding order processing, ONIX, UCP checkout,
payment, customer accounts, domain provisioning, or external fulfilment to fix
this foundation. Those remain deliberately deferred.

## Findings

| Severity | Location | Finding | Concrete bounded correction |
| --- | --- | --- | --- |
| High | `services/provider/internal/persistence/database.go:134-146`; `deploy/local/init/roles.sql:3-8` | `CheckRuntimeRole` verifies the role name and a few role flags, but it does **not** prove the promised non-owner boundary. A future migration could transfer ownership of a tenant table, schema, or database to `ocp_app`; a future role grant could also let this otherwise restricted login assume an administrative role. The API would still pass its startup check. Current local object ownership appears correct because migrations run as `ocp_dev`, but that is convention rather than an executable safeguard. | Extend the startup check to reject a runtime role that owns the current database, either application schema, or any table in `auth`/`commerce`; require the isolated initial runtime role to have no explicit role memberships (or precisely document and check an allowlist). Add integration assertions confirming `ocp_app` owns none of those objects and cannot become the migration role. Keep migrations as `ocp_dev` and retain the non-owner role test. |
| Medium | `contracts/openapi/v1/paths/store.yaml:9-16`; `paths/offering.yaml:9-17`; `paths/offering-revision.yaml:16-24` | Read-only `GET` responses say “matching retries return this original status and body.” That is an idempotency-mutation guarantee, not a property this API records or needs for reads. It can mislead a client into believing a historical read result is frozen. | Remove that sentence from GET response descriptions. Retain it only on POST/PUT responses using the `Idempotency-Key` contract. |
| Medium | `services/provider/internal/persistence/migrations/00001_foundation.sql:22-32`; `internal/persistence/database.go:73-92` | The narrowly scoped `SECURITY DEFINER` design is sound in this snapshot: qualified table name, fixed search path, revoked PUBLIC execute, and no runtime grant-table reads. Its safety depends on preserving function ownership and EXECUTE grants across later migrations, but neither is currently tested. | Add an integration assertion that `ocp_app` cannot read `auth.grants`, can execute only `auth.lookup_grant(bytea)`, and that the function owner is not `ocp_app`. Maintain the explicit schema qualification and fixed `search_path`; do not broaden the function into a general credential query. |
| Medium | `services/provider/internal/httpapi/server.go:154-198`; `internal/persistence/resources.go:46-91` | The replay implementation correctly canonicalizes decoded request bodies and scopes its table key by merchant/grant/operation/resource/client key. Its contract says the complete typed request and operation context are fingerprinted, while the digest currently contains only the body and context is relied upon solely through the database key. This is safe for the two current routes but is easy to misuse when another handler is added. | Make the canonical hash input a single explicit structure containing the operation, resource, and decoded request body (merchant/grant/client key remain database scope). This makes the source match the stated rule. Preserve current successful replay, changed-body conflict, and transaction-atomic tests. |
| Low | `docs/STATUS.md:3-39` | The status document is already much improved and correctly labels the work as under review. It should not be finalized until the high finding is corrected and checks are rerun. | Record the final review disposition, exact rerun commands, and their actual results after correction. Keep all unimplemented boundaries listed. |

## Confirmed acceptance evidence

The following parts are fit to retain after the blocker is fixed:

- The contract layout has a readable entry document, resource-path files,
  domain-owned Food schema, reusable parameters/responses/security definitions,
  and fictional checked examples. It avoids a catch-all shared-schema file.
- `cmd/contracts` prohibits remote/out-of-root references, validates both the
  modular and serialized contract, and code generation drift is checked without
  editing the committed output. The local resolver workaround is bounded and
  documented.
- The API validates the OpenAPI request shape and then applies server-side Food
  semantics; installed model lookup rejects an unknown model. Manual item
  availability is represented without pretending to reserve inventory.
- Merchant/action authorization occurs before use of a private route. A revoked
  or expired grant cannot retrieve an earlier replay result. The app role does
  not read the credentials table directly.
- `SET LOCAL` tenant context is used inside each transaction, composite tenant
  keys are present on offerings/replays, and the integration suite checks RLS
  denial and connection-pool cleanup after both rollback and commit.
- Store creation and offering append commit their replay response in the same
  transaction. Row locking plus `base_revision` provides the stated optimistic
  draft-head behavior; historical revisions are not mutable by the runtime role.
- The task workflow supplies a separate disposable PostgreSQL service for
  integration tests, starts/migrates the development database explicitly, and
  does not add CI, cloud resources, PSP credentials, or a fake network runtime.

## Accept/defer criteria

Accept Step 2 when the high finding is corrected, the two medium hardening items
are either corrected or explicitly deferred with a narrowly justified issue, and
`task check` plus `task smoke` pass on the corrected snapshot. Preserve the
generated-code comparison and real PostgreSQL integration run in that final
record.

It remains appropriate to defer public signup/OAuth, customer/API-agent flows,
purchase terms, catalog publication, UCP/Beckn runtime adoption, payment,
inventory allocation, custom-domain/DNS handling, external fulfilment, cloud
selection, and independent-provider operation. None is needed to establish this
merchant-authorized contract-and-persistence foundation.

## Verification and disposition

Verified 21 October 2026, Europe/Berlin, against the corrected uncommitted
snapshot. The previous high finding is resolved:

- `CheckRuntimeRole` now rejects a runtime role with explicit memberships,
  reachable elevated roles, or ownership/membership of the current database or
  the `auth`/`commerce` schemas and objects. The integration test temporarily
  transfers a table to `ocp_app`, temporarily grants it the administrative role,
  confirms rejection in both cases, restores the state, and confirms acceptance.
- The credential lookup test now verifies SECURITY DEFINER ownership separate
  from the app role, the fixed `pg_catalog` search path, revoked PUBLIC execute,
  app execute permission, and continued inability to read `auth.grants`.
- Replay fingerprints now explicitly serialize operation, resource, and the
  canonical decoded request. The database key continues to scope merchant,
  grant, operation, resource, and client key.
- The misleading idempotency wording was removed from read/liveness responses.

The reviewed corrected snapshot passed `task contracts:check`,
`task test:integration`, `task test:race`, and `task build` in Docker. The first
two commands establish the modular-contract/generation and actual PostgreSQL
evidence; the race suite includes the contract, HTTP, and integration packages.

**Final engineering disposition: accept Step 2 at its stated foundation scope.**
The remaining deferred work is deliberate and listed above; it is not a blocker
to committing this contract-and-persistence slice. Subsequent work must retain
the runtime-role guard and extend its assertions when adding schemas, tables,
roles, or SECURITY DEFINER functions.
