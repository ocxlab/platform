# Checkout and pickup engineering review

Reviewed 22 October 2026, Europe/Berlin (UTC+02:00), against the working tree
after `dbf340d`. Automated static review by the existing engineering reviewer;
the main agent owns the Docker verification runners. This is not an independent
implementation or a real payment-processor test.

The separate development checkout keeps guest authority in the scoped HttpOnly
cookie, reads current authorized order state, and submits revision-checked,
idempotent financial mutations. Navigating back to the receipt does not pay.
Stale forms cannot overwrite cancellation or later payment progress.

The pickup API uses merchant `order:manage` and the existing per-order lock.
It requires accepted, paid, open pickup orders and advances preparation,
readiness, and collection in sequence. Agreed terms remain immutable.

The reviewer identified a source-ownership issue: the generic event helper lived
in the development payment file. It was moved to `food_order_events.go` so the
Food lifecycle does not depend on a simulator's source ownership. Recommended
tests were added for stale checkout URLs, retained action replays, and concurrent
preparation versus cancellation. No remaining material static blocker was found.

For hydration, the reviewer correctly distinguished hypotheses from evidence.
Neither the initial event-spread hypothesis nor the Lucide-prebundle hypothesis
alone establishes a root cause. Both built and Vite browser journeys must execute
client interactions and detect hydration warnings/page exceptions; a root marker
alone is not sufficient. Runtime results are recorded in `../STATUS.md`.

The final storefront-form follow-up was also statically reviewed. Native named
controls replace hydration-dependent hidden JSON, and the initial client effect
no longer resets a user's selections on mount. The bounded form projection
accepts no client price; Go validation remains authoritative for availability,
option cardinality, duplicate options, and pricing. No concrete blocker was
found. This reviewer did not execute the shared runtime tests.

Redundant bindings were subsequently removed from native variant, extras, and
quantity controls. An item/publication-keyed block resets those controls together
when the offering actually changes. The reviewer found no blocker in that
follow-up. Without JavaScript the initially displayed item can be quoted, but
changing the item cannot dynamically repopulate its variants/options; the API
safely rejects mismatched selections. Full no-JavaScript multi-item ordering is
not claimed.

The Vite end-to-end run also exposed that installed SvelteKit 2.70.3 skips its built-in
form-origin check in development (`src/runtime/server/respond.js`,
`if (!__SVELTEKIT_DEV__)`). A small early web hook now mirrors the production
unsafe-form MIME/method/origin boundary in development as well. Static review
found no blocker: missing, null, and foreign origins fail closed, same-origin
forms pass, and Go bearer APIs remain outside this cookie-authenticated web
boundary. The production framework check remains enabled. Reverse-proxy origin
canonicalization requires intentional configuration when deployment is selected.
