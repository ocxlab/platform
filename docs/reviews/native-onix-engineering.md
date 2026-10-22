# Native ONIX catalog integration — engineering review

Reviewed 2026-10-22. This review is a static source review of the uncommitted
native ONIX lab slice. It did not run Docker, PostgreSQL, or the native-network
test. The reported parent-run journey and race results are useful execution
evidence, but are not independently reproduced here.

## Scope and reading

The reviewed slice connects an immutable Food publication intent to a retained,
pinned ONIX publisher and crawler. A signed OCP projection crosses the native
catalog boundary, and the owned discovery index validates the signature,
enrollment, model projection, and per-store generation before storing a public
catalog. This is a bounded local lab, not public federation, general peer
enrollment, Beckn checkout, or a production worker deployment.

The following parts were inspected: `internal/networkcatalog`, durable work,
`00017_discovery_catalogs.sql`, `cmd/network-lab`, the network compose profile,
and the native/catalog integration tests.

## Findings

### Resolved before final review: duplicate observations refresh freshness

`discovery.accept_catalog` returns on an identical generation, digest, and body
without updating `discovery.catalogs.observed_at`. `Index.Search` hides a catalog
after 24 hours. Consequently, a healthy crawler that repeatedly receives the
same valid signed catalog can still lose it from discovery merely because no
new publication generation was created.

The implementation now updates `observed_at` on the exact idempotent replay path
while retaining `false` to mean “no catalog content changed.” The catalog test
ages then re-accepts the same signed snapshot and verifies discovery recovers.

This is not a heartbeat protocol. It refreshes discovery only when an
authenticated duplicate reaches the owned sink. If the retained native crawler
does not repush unchanged catalogs, the 24-hour stale-hide policy remains a
conservative availability limit until active unchanged-catalog revalidation is
designed and tested.

### Resolved before final review: native mounts are scoped per participant

Each ONIX provider and the crawler mount the full
`working/native-onix` directory. That directory contains `state.json` with
merchant tokens and private signing keys, plus the other provider's config.
Thus provider A, provider B, and the crawler can read one another's credentials.
This defeats the lab's intended per-provider key boundary even though the files
are ignored and container privileges are dropped.

The composition now mounts each provider's configuration read-only with only its
own writable catalog directory, and gives the crawler only its configuration.
The broader state mount remains with the lab process and explicit test harness,
which need the synthetic credentials. This fixes the identified cross-peer file
exposure within the bounded local composition.

## Satisfactory properties observed statically

- The catalog intent is an immutable public-only payload; hidden/suspended
  publications retain `visible: false` and a `null` document. This is the right
  treatment for a suspended merchant publishing a new revision.
- The sink requires an enrolled binding, verifies the explicit Ed25519
  projection signature, validates the installed Food model before storage, and
  protects same-generation content from replacement.
- The acceptance function serializes a store's generations with an advisory
  transaction lock. Older snapshots cannot overwrite newer catalog state.
- The work seam keeps the adapter call outside the database transaction and uses
  a durable, stable effect identity. This supports at-least-once delivery; it
  does not claim exactly-once native publication.
- The native journey appropriately tests authoritative quote retrieval after
  discovery, stale-generation rejection, cross-provider routing failure, and a
  withdrawal tombstone. It is clearly separated from ordinary integration tests.

## Bounded disposition

The static blockers were corrected. Runtime confirmation remains pending the
parent-run fresh native rerun. Subject to that evidence, the slice is a useful
Food-only native publication/crawl experiment with conservative 24-hour stale
hiding. It still does not establish public federation, production key
management, a liveness protocol for unchanged catalogs, separate API/worker
deployment, or multi-domain operation.
