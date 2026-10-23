# Domain boundaries and runtime admission

The provider keeps shared merchant/account and offering-publication operations
in core, while each commerce domain owns its price, allocation, and lifecycle
decisions. Core and Food normally run together, without internal HTTP calls.
They can also run as separate processes against the same database; Shopping and
Lodging retain their separate, internal-only APIs. A domain name in configuration
does not install code or establish UCP compatibility.

## Source ownership and deployment composition

The owned Go module lives in `services/platform/`, alongside `services/onix/`.
`internal/infra/` groups technical adapters: database setup/migrations, telemetry,
HTTP runtime utilities, and object-storage/image processing. Commerce behavior
and public catalog projections remain in their business modules. This hierarchy
does not introduce extra Go modules, containers, brokers, or remote calls.

There are exactly seven immediate `internal/` directories: `core`, `food`,
`shopping`, `lodging`, `payment`, `discovery`, and `infra`. Core contains shared
application operations, Offering contract identifiers/dispatch, UCP helpers,
update streams, and catalog work. Food contains its menu implementation, purchase
persistence, and merchant MCP adapter. Cross-domain HTTP composition lives under
`cmd/api/transport/`, with process configuration under `cmd/api/config/`.
Shared tests and development-only provisioning/fixtures live under `tests/`.
Business packages cannot import those transports or development helpers; a layout
test rejects new peer directories under `internal/`.

| Source package | Responsibility |
| --- | --- |
| `internal/core/persistence` | Merchant/provider grants, store records, catalog revisions/publication, media records, authorized replay, and durable catalog intents |
| `internal/food/persistence` | Guest checkout admission, quotes, pickup orders, acceptance, fulfilment, history, and Food's development-payment coordination |
| `internal/shopping` | Shopping catalog, stock, checkout sessions, and orders |
| `internal/lodging` | Property catalog, stay pricing, booking sessions, and allocation |
| `internal/payment` | Shared typed payment subjects/terms, payment transition rules, and pay-later obligations |
| `internal/discovery` | Public catalog delivery, discovery projections, and search |
| `internal/infra/database` | Connection budgets, tenant transaction setup, restricted-role checks, and the ordered Goose migration history |

The existing Offering registry and its Food model implement menu validation,
pricing, and public projection. Core's model-bound catalog API uses that installed
model; it does not run Food's order lifecycle. REST and the current local merchant
adapter remain transport compositions over these packages. Future merchant and
customer account operations belong to core, including their eventual MCP tools;
this refactor does not implement those operations.

All seven query owners keep separate `persistence/{queries,sqlgen}` directories,
including core, Food, Shopping, Lodging, internal UCP, payment, and discovery.
Internal UCP's credential/replay queries sit beneath its composed HTTP adapter
at `cmd/api/transport/ucpinternal/`; discovery owns its queries under
`internal/discovery/`.
They read the same ordered Goose history under `infra/database/migrations` and
do not share generated Go types.
Handwritten application types and an explicitly passed SQL transaction remain
the boundary. Food reads a published catalog through core's transaction-scoped
helper, preserving the store lock that coordinates publication and placement.
Moving persistence code does not add a remote call inside that transaction.

`-service combined` is the default public composition. `-service core` mounts
shared/model-bound APIs; `-service food` mounts Food operations, including guest
checkout admission. Both retain health endpoints. Opposite-service routes return
the contracted 503 Problem before credential lookup or database access. A future
split deployment can route the same URL paths to their owners at ingress;
the current web development setup continues to use the combined API. No split
ingress or distributed-database transaction is claimed here.

`-service shopping` and `-service lodging` select one internal domain each.
Contradictory domain/service configuration fails startup. The role flags below
remain useful for internal test composition and the catalog worker. Each process
has its own bounded pool. Core starts only the offering-update listener; Food
starts only the order-update listener. The combined process starts both. The
database, deployed image, and any underlying machine remain shared infrastructure,
not independent capacity or failure domains.

For optional local split-process checks, `task services:dev` starts core on
localhost:18182 and Food on localhost:18183. `task services:stop` stops only these
two processes and retains data. Regular `task dev` / `task web:dev` still use the
combined API on port 18180. No mandatory extra container, broker, database, or
cloud product is introduced. Source dependency and route-admission tests guard
these boundaries; `task test:domains` also exercises actual runtime processes.

## Domain admission

`OCP_ENABLED_DOMAINS` selects domain-specific HTTP operations for an API process.
Absent configuration preserves Food. `food` explicitly selects Food, and `none`
retains shared store/account operations without catalog or purchase operations. Unknown,
duplicate, and uninstalled selections fail startup. `-role internal` instead
selects Shopping/Lodging and defaults to both; an explicit single-domain value
permits independent API processes. `-role worker` currently selects only Food
catalog publication work with an explicitly mounted publisher enrollment/key
configuration. See [internal commerce](INTERNAL-COMMERCE.md).

Every path declares `x-commerce-boundary` as `shared`, a recognized domain name,
or `model` for model-bound catalog operations. The runtime rejects missing or
unknown classifications at startup and rejects disabled domain operations
before authentication, database access, or business dispatch, using the existing
contract-defined 503 Problem response. This is process admission, not a replacement
for actor authorization. The model registry includes only enabled implementations,
registered by exact model identifier/version. With no enabled model, catalog reads,
writes, publication, public listings, handles, media, guest-session creation, and
offering streams reject before storage. Shared store/account management remains
available. The current Food-specific local merchant MCP
is not mounted in a process with Food disabled.

HTTP instrumentation labels these routes with bounded `ocp.domain` values; shared
routes use `shared`. Merchant/store/order IDs, payloads, and credentials are not
metric labels. These signals can later populate provider administration without
making the dashboard a dependency of request handling.

Payment has a typed internal subject and immutable amount/currency projection.
Food validates that projection after reading the authoritative order terms;
callers cannot select another domain or supply payment amounts. The existing
development payment table remains Food-specific and synchronous. This type is
not a shared charge ledger, payee-enrollment mechanism, or real adapter.
Shopping/Lodging reuse the subject/terms validation and write an explicit
outstanding pay-later obligation in their completion transaction. No charge is
invented to demonstrate this shared seam.

Selective API/worker roles use the same maintained binary/image. Each domain
owns its records and allocation algorithm. Process concurrency and database pools
are bounded independently. Food publication commits a durable immutable outbox
intent; leases fence worker completion, and repeated delivery retains the same
external catalog identity. No broker or Kubernetes deployment is required.
Executed role/recovery results and limitations are recorded in `STATUS.md`.
One initial PostgreSQL cluster still shares capacity and failure modes.

The public offering registry recognizes the current exact Food V1 model; public
listing projection filters by installed models. Internal Shopping/Lodging catalogs
have their own paths and tables, rather than exposing unsupported model rows
through public Food routes. This is deliberate ingress separation, not a claim
that arbitrary future models can already mix safely in every public API.
