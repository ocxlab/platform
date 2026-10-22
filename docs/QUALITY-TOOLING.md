# Contract exploration and Go quality tools

The three tools serve different purposes: Schemathesis explores HTTP contracts,
sqlc turns owned SQL into typed Go calls, and golangci-lint checks Go source for
common defects. They supplement the existing behavioral, PostgreSQL, browser and
mutation tests; none replaces those tests or proves interoperability.

## Running the checks

| Command | Purpose | Boundary |
| --- | --- | --- |
| `task test:contracts` | Run bounded Schemathesis exploration and an authenticated Food journey | Disposable Compose database and private test API only |
| `task sql:generate` | Generate query methods from SQL and migrations | Ignored Go output, no database connection |
| `task sql:check` | Parse queries and compare a fresh generation | Temporary output compared with current generated files |
| `task lint:go` | Verify configuration and run focused Go linters | Handwritten Go, after code generation |

`task check` includes SQL generation checks and Go linting. Schemathesis is a
separate, explicit task because it recreates the disposable test database. Run it
serially with integration and browser tasks, which share that database. It never
targets the development database or accepts an arbitrary public API URL.

All three run in pinned Docker images; no host Go or Python is required. Versions
selected on 23 October 2026 are Schemathesis 4.32.4, sqlc 1.31.1 and golangci-lint
2.15.0. Image digests are recorded in Compose and the provider Dockerfile. Updates
should change the pin deliberately and rerun these checks, not float a tag.

## Schemathesis exercises reachable application state

The harness in `contracts/testing/test_api.py` loads the generated OpenAPI bundle.
It provisions synthetic credentials into the disposable database, then creates and
publishes a menu, obtains a guest credential, prices a valid selection and places
an order. Merchant and guest reads validate the resulting order; explicit denied
requests exercise missing credentials and a different merchant scope.

The subsequent allowlist explores both schema-valid and schema-invalid inputs.
Real merchant/store path parameters and appropriate credentials keep requests from
merely collecting unauthorized/not-found responses. Each generated mutation gets
a fresh idempotency key. The existing deterministic tests remain responsible for
replay, exact terms, concurrency and detailed domain rules.

Checks cover unexpected server errors, declared status codes, response content
types, headers and response schemas. A schema-valid request is not necessarily a
business-valid order; a declared 4xx is not proof of a defect or successful domain
behavior. The seeded positive journey separately requires specific success codes.
The ignored `reports/contracts/schemathesis.json` records actual response counts
and status codes, not coverage percentages. Failed runs leave `passed: false`.

This initial bounded harness does not explore every endpoint, SSE, object uploads,
external callbacks, real payments or the internal Shopping/Lodging processes.
Extend the allowlist with deliberate state/authorization setup, rather than
pointing unconstrained fuzzing at production. Keep credentials and minimized
failure requests private; reports are not publication artifacts.

The initial run found an escaped NUL in a store name reaching PostgreSQL and
returning 503. The correction is contract-first: reusable `PlainText` in
`contracts/openapi/v1/schemas/common/text.yaml`, composed into store names, Food
names/translations, customer notes and search inputs. The OpenAPI validator checks
declared input constraints first. A subsequent generic JSON guard also checks
keys and opaque document content, which the outer offering envelope cannot fully
constrain. Regression tests cover the schema itself and the actual HTTP boundary;
the latter must return 400, not report a storage outage.

## sqlc retains application transactions

The adoption now covers 16 queries grouped under
`services/provider/internal/persistence/queries/`: store creation, lookup and
pagination; offering head locking, append and lookup; publication reads, visibility
updates, audit and catalog snapshots; and durable effect insertion/deduplication.
The config reads all Goose migrations in ascending order. sqlc generates a private `sqlgen` package;
it does not own migrations or introduce an ORM, new connection pool or cloud service.

Handwritten persistence code calls `sqlgen.New(tx)` with its existing tenant-scoped
transaction. Row-level security, tenant predicates, `FOR UPDATE`, optimistic
generation checks, audit entries and transactional outbox work remain intact.
Generated database types stay behind handwritten domain/HTTP types.

Edit SQL/configuration, then run `task sql:generate` and `task check`. Never edit
or commit `internal/persistence/sqlgen/`. Task and clean Docker builds generate
it themselves. `sqlc vet` here is the local parser/check command; no remote
analysis or database-backed EXPLAIN rules have been configured. PostgreSQL tests
remain necessary to establish runtime locking and isolation behavior.

Nullable JSON uses a configured standard-library `*json.RawMessage` mapping, not
an additional generated-code runtime dependency. The store-list query uses an
internal negative count sentinel for non-Food offerings; the handwritten mapper
preserves the API's null count. Nonnegative counts still describe empty/new or
Food menus. Runtime tests cover these mappings, including publication withdrawal.

Adoption remains incremental: purchase/payment transitions, grants, replay-table
selection, worker lease functions, and other persistence paths still use handwritten
SQL. Generated code reduces parameter and row-scan mismatches; it does not decide
whether a query is authorized or a business transition is correct. The
[database workflow](DATABASE-WORKFLOW.md) explains how reviewed Goose migrations,
pgschema drafts, Squawk, and sqlc fit together.

## Go linting focuses on correctness

The root `.golangci.yml` explicitly enables govet, staticcheck, unused,
ineffassign and errcheck. Style and quick-fix suggestions are not part of this
initial gate. Strict generated-file detection excludes generated source, not
handwritten handlers. No historical findings baseline is hidden in the config.

Errcheck remains enabled for production code. Its three global exceptions are
SQL teardown (`DB.Close`, `Tx.Rollback`, `Rows.Close`); query, commit and row
iteration errors must still be checked. Test files currently exclude errcheck
because fixture cleanup often intentionally discards errors. Other intentional
discards are explicit at the call site. Narrow these exceptions if the codebase
later adopts stricter teardown diagnostics.

## Official references

Consulted 23 October 2026. These explain upstream behavior; repository choices
and executed outcomes are documented above and in `STATUS.md`.

- [Schemathesis Docker guide](https://schemathesis.readthedocs.io/en/stable/guides/docker/): image usage and version pinning.
- [Schemathesis Python reference](https://schemathesis.readthedocs.io/en/stable/reference/python/): schema operations, strategies and case validation.
- [Schemathesis checks](https://schemathesis.readthedocs.io/en/stable/reference/checks/): response and server-error checks.
- [sqlc configuration](https://docs.sqlc.dev/en/latest/reference/config.html): version 2 SQL inputs and Go generation.
- [sqlc migrations](https://docs.sqlc.dev/en/v1.31.1/howto/ddl.html): migration directory interpretation.
- [sqlc transactions](https://docs.sqlc.dev/en/v1.31.1/howto/transactions.html): generated queries with transactions.
- [sqlc type overrides](https://docs.sqlc.dev/en/v1.31.1/howto/overrides.html): nullable mappings and pointer types.
- [golangci-lint configuration](https://golangci-lint.run/docs/configuration/file/): explicit linters and exclusions.
- [Linter settings](https://golangci-lint.run/docs/linters/configuration/): errcheck function exclusions and staticcheck checks.
