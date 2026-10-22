# Domain boundaries and runtime admission

The provider keeps a shared merchant/account and offering-publication foundation,
while each commerce domain owns its price, allocation, and lifecycle decisions.
Food is currently the only installed domain. A domain name in configuration does
not install code or establish UCP compatibility.

`OCP_ENABLED_DOMAINS` selects domain-specific HTTP operations for an API process.
Absent configuration preserves Food. `food` explicitly selects Food, and `none`
retains shared store/account operations without catalog or purchase operations. Unknown,
duplicate, and uninstalled selections fail startup. Shopping and Lodging cannot
yet be enabled. Their contracts and implementations must precede installation.

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
not yet a shared payment ledger, a payee-enrollment mechanism, or a real adapter.
Those require verified domain terms references, payment-operation identity, and
durable effect/reconciliation contracts before another domain uses them.

The growth target is selective domain API/worker processes from the same image,
with stable routing contracts and independently bounded work. The initial admission
mechanism is only the first part. Domain persistence ownership, transactional
outbox, worker lease fencing, internal-only UCP ingress, independently configured
connection limits, and multi-process recovery/contention tests remain to implement.
One initial PostgreSQL cluster continues to share capacity and failure modes.
Before installing a second domain, model-bound reads and writes must resolve each
resource's exact model against the enabled registry; mixed public listings must
filter by supported models. The empty-registry route gate is sufficient only for
this currently Food-only installation, not evidence of mixed-domain isolation.
