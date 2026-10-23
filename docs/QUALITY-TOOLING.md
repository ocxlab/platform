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

Application query adoption now covers 103 named statements in seven owner-specific
`internal/<owner>/persistence/queries/` directories. Related concerns have separate
files: for example, Shopping separates catalog, checkout sessions, and orders;
Lodging separates properties, booking sessions, and allocations. Core owns shared
credentials, stores, publication, media, replay, and durable work access. Food owns
checkout admission, quotes, orders, and development-payment records.

| Query owner | Named statements |
| --- | ---: |
| Core | 45 |
| Food | 25 |
| Shopping | 12 |
| Lodging | 13 |
| Internal UCP transport | 5 |
| Shared payment | 1 |
| Network catalog | 2 |

The config reads all Goose migrations in ascending order from
`internal/infra/database/migrations`. Each owner generates a private, ignored `sqlgen`
package. No generated type crosses the handwritten application/HTTP boundary.
This does not introduce an ORM, connection pool, runtime dependency, or deployment.

Handwritten application code still passes its existing transaction to
`sqlgen.New(tx)`. Tenant context, explicit tenant/customer/guest predicates, row
locks, optimistic revisions, audit entries, payment obligations, and outbox work
remain within their original transaction boundaries. Separate public-offering
read/lock statements preserve store serialization during checkout. Food order
reads still acquire the operational-row lock before reading payment state; they
do not replace that sequence with a potentially stale pre-lock join.

Replay uses separate generated statements for merchant, provider, and guest
tables, selected by the authorized principal. Internal UCP replay stays with its
transport owner. Advisory-lock identity, fingerprinting, conflict checks, and
committing the response with its decision remain handwritten orchestration.
Historical REST grant statements deliberately do not mention the later resource
column; MCP grant lookup/provisioning remains resource-bound, without REST fallback.

Edit SQL/configuration, then run `task sql:generate` and `task check`. Never edit
or commit an owner's `persistence/sqlgen/`. Task and clean Docker builds generate
all seven packages. Determinism checks recreate every owner in temporary output.
`sqlc vet` here is the local parser/check command; no remote analysis or
database-backed EXPLAIN rules are configured.

Nullable JSON uses the standard-library `*json.RawMessage` mapping. Media
dimensions map database NULL to absent API fields until completion. Food inbox
queries map the optional development-payment join to the same payment state and
provider fields as the detailed order read. Lodging date inputs remain validated
date strings, explicitly cast to PostgreSQL dates; allocation overlap rules and
deterministic unit locks remain unchanged. Worker claim results require a lease
token and expiry, and completion retains database-enforced lease fencing.
Worker domain lists are encoded as JSON and converted to a PostgreSQL array in
the query, avoiding an additional array-codec dependency.

The replaced inline statements, positional argument plumbing, and scanning loops
have been removed from callers, rather than retained as parallel implementations.
No entire handwritten source file became obsolete: validation, pricing, locking
order, transaction orchestration, and response mapping still need those files.
Code generation checks types and SQL structure; behavioral PostgreSQL tests are
still required for authorization, concurrency, rollback, and agreement semantics.

### Remaining handwritten SQL inventory

The 23 October 2026 follow-up audit finds two normal runtime execution call sites,
down from 66 before this expansion: transaction-local tenant context and the
restricted-role startup check, both in `internal/infra/database/database.go`. They are
connection/security infrastructure, intentionally independent of domain query
packages. Six administrative demo-data setup calls remain in
`internal/devsetup/restaurant.go`; they are not customer/merchant runtime paths.
The SSE listener's pgx `LISTEN` command, executable bootstrap SQL, test fixtures,
and SQL inside Goose migrations/functions are outside this application-query
inventory. These are call-site counts, not distinct-query counts or percentages.

`internal/architecture/sql_ownership_test.go` rejects direct
`ExecContext`, `QueryContext`, `QueryRowContext`, and `PrepareContext` calls
outside generated output, database setup, and development setup. A positive
control checks calls with variable statements too. The guard covers those
database/sql call forms; it is not a general proof against every possible SQL
execution mechanism.

See the [database workflow](DATABASE-WORKFLOW.md) for how reviewed Goose
migrations, pgschema drafts, Squawk, and sqlc fit together. This batch changes
query access, not migration history or the event/SSE architecture. Go channels,
Watermill, and modular-monolith event delivery remain a separate discussion.

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
