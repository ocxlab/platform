# Reviewing database changes

Goose remains the deployment history and migration executor. pgschema helps draft
structural changes, Squawk checks migration SQL for hazards, and sqlc generates
typed Go query methods from the reviewed history. This keeps generation useful
without making a schema diff responsible for tenant security or preserving data.

All commands run through Task and Docker. They need no host PostgreSQL, Python,
or Go installation. The review database is a separate, unexposed, disposable
PostgreSQL service; these commands never connect to the development database.
The review image currently targets Linux amd64; ARM hosts need Docker emulation.
This tooling choice does not constrain the application runtime's architecture.

## Draft a change

1. Run `task db:schema SCHEMA=commerce NAME=add-store-label`. It replays the actual
   Goose migrations into a fresh database and exports
   `working/migrations/add-store-label/desired.sql`. An existing desired file is
   never overwritten. Other supported schemas include `auth`, `payment`,
   `payments`, `runtime`, `discovery`, `commerce_internal`, `shopping`, and `lodging`.
2. Edit that SQL to describe the desired schema, then run
   `task db:draft SCHEMA=commerce NAME=add-store-label`. Inspect `draft.sql` and
   `plan.json` alongside your edit. The comparison starts from a fresh Goose
   replay, not your mutable development database. A separate clone supplies
   cross-schema context to pgschema's temporary planner schema.
   Exported scope metadata prevents accidentally planning another schema or
   using stale desired SQL after migration history changes; export a new NAME
   and merge your intended edits in that case.
3. Turn the draft into a reviewed, numbered Goose migration under
   `services/platform/internal/infra/database/migrations/`. Drafts are ignored local
   work; only reviewed migration SQL belongs in Git. There is deliberately no
   automatic promotion or pgschema apply command. pgschema's JSON wait directives
   are review information, not executable Goose SQL.
4. Run `task db:lint`, `task sql:generate`, and `task check`. Add upgrade tests
   covering existing rows and denied access, not just creation on an empty database.
   Once reviewed, `task db:prepare` applies the committed migration history to
   your local development database through Goose; drafting never does that.

For related changes across schemas, export and plan each scope separately, then
review ordering and dependencies together. A desired file may use pgschema's
include directives to split tables, functions, and policies into manageable files.
For example, `\i tables/stores.sql` includes a file beneath the desired file's
directory; the round-trip test exercises such a multi-file desired schema.
These local files are drafting inputs, not a second committed schema authority;
sqlc continues to read the ordered Goose history.

The review service can be stopped with
`docker compose --project-directory . --file deploy/local/compose.yaml stop migration-db`.
It has no persistent data volume. Export and draft commands clean up their unique
databases on ordinary completion or failure; an interrupted container may leave
temporary review databases until the disposable service is recreated.

## Review what generation cannot decide

Check renames, backfills, destructive changes, grants, ownership, roles, default
privileges, identity sequences, row-level security, and security-definer functions
explicitly. The local evaluation found omissions for function ownership, schema
CREATE grants, and identity-sequence increments. A rename can become drop/add,
which loses data. These are reasons to review a draft, not accept it unchanged.

Prefer schema-qualified DDL. pgschema can emit unqualified names. An explicit
`search_path` must preserve access to Goose's version table in `public`; hiding
that table also prevents Goose from recording an otherwise successful migration.
The normal transactional form uses `SET LOCAL lock_timeout = '2s'` and a bounded
`SET LOCAL statement_timeout = '2min'`; choose actual budgets for each operation.
Concurrent index creation needs Goose's `-- +goose NO TRANSACTION` annotation and
session-level timeout settings instead. Its failure/retry behavior needs its own
upgrade test because Goose cannot roll it back atomically.

## Safety gate and historical baseline

`task db:lint` verifies LF-normalized SHA-256 hashes of the existing 18 migrations
in `scripts/migrations/baseline.json`, then strictly checks every new migration's
Up section. Existing history is not rewritten to silence its known findings.
Do not update the baseline to exempt a failing new migration. The original
research audit reported 61 findings under its then-selected rule set; the product
gate additionally enables schema-qualified table checks and RLS-change checks.

The gate wraps ordinary migrations in explicit BEGIN/COMMIT, matching Goose's
default transaction behavior; it honors NO TRANSACTION and excludes Down SQL.
It rejects unsupported environment substitution rather than linting different SQL
from what Goose would execute. Findings refer to the temporary Up projection;
transaction-wrapped line numbers have one extra leading line.

Use narrow upstream `squawk-ignore` annotations only for reviewed exceptions,
with an adjacent explanation of why the operation is safe here and the upgrade
test covering it. Do not globally disable a rule to make a migration pass.
Squawk is not an authorization verifier: for example, it did not flag a PUBLIC
grant in our earlier probe. RLS/ACL and tenant-isolation tests remain mandatory.

`task test:migrations` exercises the gate and a real Goose/pgschema round trip on
the isolated review database. Both checks are included in `task check`.

## Upstream references

Consulted 23 October 2026. These describe tool behavior, not guarantees about our
migrations. Pins live in `scripts/migrations/Dockerfile`; Goose uses the same
library version as `services/platform/go.mod` (currently 3.26.0).

- [Goose SQL annotations](https://pressly.github.io/goose/documentation/annotations/): transaction boundaries and statement blocks.
- [Squawk CLI](https://squawkhq.com/docs/cli): findings and narrowly scoped suppressions; pinned 2.70.0.
- [pgschema plan](https://www.pgschema.com/cli/plan): scoped desired SQL and external planning database; pinned 1.13.3, checksum verified in Docker.
- [pgschema source and license](https://github.com/pgplex/pgschema/tree/v1.13.3): retained tool provenance.
