# Customer Food intentions

24 October 2026. The local Customer Food MCP at `/mcp/customer/food` lets an
agent browse published dishes, price an intended basket, and manage its own
pickup order. It shares Food checkout application operations with REST. It is
not a registered-customer login, public OAuth deployment, payment integration,
or a claim of UCP Food conformance.

## Follow the customer journey

Start with `find_food_stores` and `inspect_store_menu`: these return at most
twenty summaries per page. This convenience directory includes visible stores
on this provider, not Beckn network discovery; non-Food menus are rejected when
inspected. `inspect_food_dish` gives sizes and modifier-group summaries, then
expands up to three explicitly requested groups. `inspect_food_meals` separately
discloses upgrades and paginated, named included-item choices. All these reads
use the public projection, never merchant drafts. Continuations retain the
publication generation so a changed menu cannot silently mix pages.

`price_food_basket` takes the chosen store, exact generation, and complete
selection, including sizes, options, meal upgrades, quantities, and notes. It
checks that the chosen store matches this credential's checkout store. The
result's `basket_id` identifies an immutable, owned ten-minute quote, not hidden
MCP connection state. `inspect_food_basket` retrieves its exact terms.
`revise_food_basket` accepts indexed replacements/removals and additions, retaining
unchanged selections and notes; it prices a **new** snapshot. Previous unexpired
quotes remain independently valid. There is no durable mutable cart or resource
reservation in this increment. Quote creation is non-idempotent and capacity-bound.

Before `commit_food_order`, show the exact basket and total and obtain customer
confirmation. The tool requires `confirmed`, the exact total/currency, and a retry
key. These fields guard accidental selection mismatch; they do not independently
prove human consent. The authorized client remains responsible for obtaining it.
First placement checks current store ordering policy and quote validity. Its
mutation, agreed terms, durable work, audit, and successful replay commit together.
A successful retry reads retained terms even after the quote is deleted or the
menu withdrawn. A changed total is rejected, including on replay.

`track_my_food_order` defaults to compact progress; item details and history are
opt-in. `cancel_my_food_order` requires explicit confirmation, operational revision,
and a retry key; the shared lifecycle prevents automatic cancellation after
preparation starts. These tools neither charge money nor report kitchen dispatch.
The existing development-payment UI/API remains separate.

## Local access and boundaries

Run `task dev:customer-mcp-credential STORE=store-N` after publishing a local store.
The command only provisions into `ocp_dev` and prints one new one-hour,
store-bound checkout secret. This creates a new guest actor: it does not adopt
browser purchases, exchange another guest's credential, or link accounts by name.
The local API enables this endpoint with `OCP_LOCAL_CUSTOMER_MCP=true`.
Use Streamable HTTP with that token, separately from `/mcp/merchant`.

REST guest, merchant, and provider credentials are rejected here. Customer Food
MCP credentials are rejected by REST; database resource filtering supplements
the distinct prefix. Each stateless request reauthenticates, and transactional
guest checks still apply. Localhost Host and same-origin restrictions, request
size/time bounds, and no-store responses are shared with the merchant transport.
Remote OAuth, account-backed payment methods, and cross-store customer identity
remain explicit follow-ups rather than privileges inferred from this local grant.

Bounded OTel tool labels include the surface, registered tool name, outcome, and
stable error code. They exclude credentials, arguments, menu text, and order IDs.
Runtime and verification evidence is recorded in [development status](STATUS.md).
