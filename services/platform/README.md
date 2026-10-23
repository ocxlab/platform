# Owned platform runtime

This Go module supplies the owned commerce implementation. Retained ONIX packaging
lives beside it under `services/onix/`, not inside this module. Run commands from
the repository root using Task/Docker; no host Go or PostgreSQL is required.

`internal/infra/` groups database setup and migration execution, telemetry,
HTTP runtime helpers, and object-storage/image processing. Core, Food, Shopping,
Lodging, payment, and network catalog retain their business ownership outside
that technical group. The single Go module and selectable runtime compositions
are unchanged; directory separation does not require separate deployments.

The current request path is generated HTTP routing → credential/action checks →
request validation → Offering or store operation → transaction-scoped SQL.
`internal/httpapi` adapts HTTP; `internal/offering` owns model dispatch;
`internal/offering/food` validates the initial catalog definition;
`internal/core/persistence` owns shared account, store, catalog, and replay access;
`internal/food/persistence` owns quotes, orders, and fulfilment.
`internal/infra/database` owns migration execution and connection/tenant setup. Each
query owner has its own ignored sqlc output. The Food validator does not create
purchases or model every future kind of offering.

All business SQL uses owner-specific `persistence/queries/` inputs and generated
query methods: core, Food, Shopping, Lodging, internal UCP, payment, and network
catalog. Core/Food have separate transaction-oriented application operations;
Shopping/Lodging retain their domain operations and private generated packages.
Only tenant/role setup and local demo provisioning retain direct database/sql
execution in handwritten internal code. See [quality tooling](../../docs/QUALITY-TOOLING.md).

`cmd/api` uses `OCP_DATABASE_URL` and rejects a privileged database role.
Its `-service` flag selects `combined` (the default), `core`, `food`, `shopping`,
or `lodging`. Core/Food can therefore share a cheap initial process and later
scale as separate processes without replacing their application operations.
Shopping/Lodging remain internal-only and independently selectable. Separate
processes still share the database's capacity and failure modes.
`cmd/migrate` and `cmd/provision` use `OCP_MIGRATION_DATABASE_URL` explicitly.
Compose/Task supply local-only defaults. The API never runs migrations or accepts
administrator database credentials as a fallback. Credential provisioning prints
the token once and stores only its digest. Production signup/login is not implemented.

Every data operation begins a transaction and installs its merchant context with
`set_config(..., true)`. Forced row-level security and tenant-qualified keys add
database enforcement to the application's authorization. Transaction-local context
is not a defense against arbitrary SQL execution by a compromised process;
runtime credentials cannot access the private credential table or perform DDL.

Store creation and offering append commit their successful response bytes alongside
the data mutation. An advisory transaction lock serializes the scoped retry key;
a row lock plus `base_revision` serializes competing offering heads. Immutable
revisions preserve model/version and manual availability without claiming that
items are reserved or published. Replay has no automatic expiry in this slice.

`task check` is the acceptance command. Tests start disposable PostgreSQL, migrate
an isolated database twice, and exercise real HTTP/SQL behavior using the restricted
role. They cover process-handler restart with the same database and simulated
development payments, not storage-loss recovery, production failover, or real
payment processing. `task test:domains` additionally runs compiled processes;
native network acceptance has its own `task test:network` command.
