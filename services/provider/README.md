# Owned provider runtime

This Go module supplies the owned commerce implementation. Retained ONIX packaging
lives beside it under `services/onix/`, not inside this module. Run commands from
the repository root using Task/Docker; no host Go or PostgreSQL is required.

The current request path is generated HTTP routing → credential/action checks →
request validation → Offering or store operation → transaction-scoped SQL.
`internal/httpapi` adapts HTTP; `internal/offering` owns model dispatch;
`internal/offering/food` validates the initial catalog definition;
`internal/persistence` owns migrations and database access. The Food validator
does not create purchases or model every future kind of offering.

`cmd/api` uses `OCP_DATABASE_URL` and rejects a privileged database role.
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
role. They cover process-handler restart with the same database, not storage-loss
recovery, production failover, native network interoperability, or payment.
