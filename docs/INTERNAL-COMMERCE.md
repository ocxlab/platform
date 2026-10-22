# Internal Shopping and Lodging APIs

22 October 2026. This implementation tests commerce beyond Food without making
every domain follow a restaurant order lifecycle. Shopping allocates stock;
Lodging allocates dated physical units. Both retain agreed terms and an
outstanding pay-later obligation atomically. Neither is a public launch product,
and neither currently takes card payments.

## Ownership and deployment

The public process serves Food and shared merchant/store operations. An explicitly
started internal process serves Shopping, Lodging, or both, without Food or
merchant MCP routes. Each domain can move to its own process without rewriting
its allocation rules or changing session paths.

| Module | Owned decisions | Owned records |
| --- | --- | --- |
| Shopping | Variant pricing, stock checks, checkout completion | Catalogs, stock, checkout sessions, immutable orders |
| Lodging | Stay pricing, booker requirements, unit/date allocation | Properties, units, booking sessions, immutable bookings, allocations |
| Internal transport | Credential admission, request validation, replay, bounded dispatch | Internal grants and replay responses |
| Payment | Validate server-derived financial terms and retain pay-later obligations | Obligations keyed by merchant, store, domain, and subject |

Shared merchant/store identity is reused; the domains do not join each other's
business tables. `sessionkit` shares wire records, not a universal allocation
engine. PostgreSQL remains a shared capacity and failure boundary. Moving a
domain to another database would require explicit identity/payment integration
and data migration; that is not a configuration-only promise.

## Implemented operations

Paths are relative to the internal listener. The selected UCP REST service
endpoints are `{origin}/shopping` and `{origin}/lodging`.

| Operation | Shopping | Lodging |
| --- | --- | --- |
| Replace catalog | `PUT /shopping/catalog` | `PUT /lodging/catalog` |
| Create session | `POST /shopping/checkout-sessions` | `POST /lodging/booking-sessions` |
| Read session | `GET /shopping/checkout-sessions/{id}` | `GET /lodging/booking-sessions/{id}` |
| Replace selections | `PUT /shopping/checkout-sessions/{id}` | `PUT /lodging/booking-sessions/{id}` |
| Complete session | `POST /shopping/checkout-sessions/{id}/complete` | `POST /lodging/booking-sessions/{id}/complete` |
| Cancel unfinished session | `POST /shopping/checkout-sessions/{id}/cancel` | `POST /lodging/booking-sessions/{id}/cancel` |
| Read retained order | `GET /shopping/orders/{id}` | Completed booking session retains confirmation |

Catalog replacement and Shopping order reading are owned internal APIs, not
upstream UCP catalog or Order API claims. Catalog writes require a separate
management credential and expected `base_revision`. Customer credentials cannot
edit catalogs; management credentials cannot buy for customers.

Creation prices selections without reserving resources. Completion rechecks
expiry and catalog revision. Changed prices require a session update, not silent
repricing. Shopping locks selected stock rows in stable order. Lodging locks
physical units and also uses a PostgreSQL exclusion constraint on half-open
date intervals: one guest's checkout day permits another guest's check-in.

Sessions last 15 minutes and hold no resources, so expiry/cancellation has nothing
to release. Cancelling a session does not cancel a confirmed order or booking.
Post-confirmation cancellation, returns, and refunds are outside this internal
subset, not fabricated UCP operations.

Completion explicitly selects the configured instrument:

```json
{"payment":{"instruments":[{"id":"pay-later","handler_id":"ocp-pay-later","type":"pay_later"}]}}
```

It records `pay_later` / `outstanding`, not successful payment. It neither enrolls
a real payee nor turns Food's development-payment adapter into a real
Shopping/Lodging payment integration.

## Selected UCP compatibility

The target is the draft source snapshot
[`d13cc61455d2a90f7e6038df07fdbe72dcccdcc7`](https://github.com/Universal-Commerce-Protocol/ucp/tree/d13cc61455d2a90f7e6038df07fdbe72dcccdcc7).
Its dated wire version is `2026-08-25`; matching that string does not establish
stable-release conformance. Research source IDs CE-U12/CE-U13 identify Shopping
checkout/Lodging booking; CE-U14/CE-U15 identify their REST bindings.

| Concern | Implemented selection | Check |
| --- | --- | --- |
| Headers | UUID `Request-Id`, mutation UUID `Idempotency-Key`, `UCP-Agent` profile dictionary | Generated client and HTTP negatives |
| Profiles | Versioned service/capability arrays and selected handler instance | Server-owned internal platform fixture only |
| Session payloads | Shopping lines; Lodging property, stays, rates, occupancy, booker | Actual responses validated against exact-pinned upstream schemas |
| Completion | Shopping order and Lodging booking confirmations | Atomic allocation, immutable terms, outstanding obligation |
| Failures | Problem JSON and bounded inputs | Owned OpenAPI request/response validation |
| Replay | Canonical payload digest scoped to grant/domain/operation/resource/key | Exact replay across replicas/restart; changed payload rejected |

The platform profile URI and document are fixed fixtures. Credentials are opaque,
hashed, expiring, revocable, and bound to internal audience, merchant, store,
domain, customer, and management authority. No request triggers arbitrary
profile/schema fetches. This restricted cohort is not general UCP authorization
or a complete Structured Fields parser. The advertised business payment schema
URI uses the configured origin and has a matching authenticated route.

Optional upstream tests fetch only fixed pinned primary-source paths into memory,
log SHA-256 values, and reject a deliberately invalid response. They do not run an
independent UCP product or certify every extension, transport, currency, or handler.

## Local use and verification

`task test:domains` builds the maintained Linux binary in Docker, runs real
separate API/worker child processes and disposable PostgreSQL journeys, and
enables pinned upstream schema checks. The interrupted worker's HTTP receiver is
a test double; `task test:network` separately uses native ONIX containers.

`task domains:dev` starts optional roles from the same API image. Food retains its
normal local port; internal APIs and the catalog worker publish no ports. Internal
roles have local 128 MiB / 0.5 CPU limits and two database connections each. These
are test budgets, not production capacity recommendations. The worker uses an
explicit mounted enrollment/key file, owner-readable only by its runtime UID.

Choose a store with `task dev:stores`, then print a one-hour fixture credential:

```text
task dev:internal-credential STORE=store-1 DOMAIN=shopping MANAGEMENT=true
task dev:internal-credential STORE=store-1 DOMAIN=shopping
task dev:internal-credential STORE=store-1 DOMAIN=lodging
```

The explicit output contains identity and token; never commit it. Optional
`CUSTOMER=<UUID>` reuses a fixture customer, otherwise each invocation creates
a new customer identity. These grants are local administration, not registration
or OAuth. A trusted client on the Compose network can call `internal-shopping:8080`
or `internal-lodging:8080`. The modular contract is `contracts/openapi/internal`;
its bundle and generated client are ignored artifacts, not tracked source.

`task domains:stop` stops optional roles while retaining data. `task network:stop`
separately stops the native lab. Final results are in `docs/STATUS.md`; static
reviews are under `docs/reviews/internal-domains-*`.
