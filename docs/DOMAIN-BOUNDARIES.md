# Domain boundaries and runtime admission

The provider keeps a shared merchant/account and offering-publication foundation,
while each commerce domain owns its price, allocation, and lifecycle decisions.
Food is installed on the public listener. Shopping and Lodging are installed on
an explicitly separate internal listener; their catalogs and sessions are not
inserted into Food's offering registry. A domain name in configuration does not
install code or establish UCP compatibility.

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

The public offering registry recognizes exact Food V1/V2 model versions; public
listing projection filters by installed models. Internal Shopping/Lodging catalogs
have their own paths and tables, rather than exposing unsupported model rows
through public Food routes. This is deliberate ingress separation, not a claim
that arbitrary future models can already mix safely in every public API.
