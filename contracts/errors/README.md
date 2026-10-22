# Public error codes

API consumers recover from stable `Problem.code` values, not English text or Go
error strings. Each JSON file owns one module's codes. Shared transport, access,
and replay failures belong to common; offering, Food selection, and checkout
failures stay with those responsibilities. Codes are unique across the API and
keep their meaning; do not reuse a code for a different outcome.

Each entry defines its HTTP status, safe developer-facing catalog label, and Fluent key.
The wire title uses the HTTP status phrase because the current type is
`about:blank`, following RFC 9457 section 4.2.1. The specific category is `code`.
The existing snake_case wire codes are retained. The wire schema leaves codes
open for additive evolution; an older application displays a generic message for
an unknown code, never a backend detail or exception. The server keeps safe English
details for API diagnosis. Neither title nor detail is a localization contract.
Internal Go sentinel errors remain private and are mapped at the HTTP boundary.

Run `task errors:generate` after edits. It validates duplicate codes, ownership,
status ranges, and message-key naming, then produces the embedded Go catalog and
TypeScript registry. `task errors:check` runs under `task check` to detect drift.
The HTTP boundary verifies code/status consistency and falls back to a safe
unavailable error for an invalid mapping; tests also check literal call sites.

The web app uses the official `@fluent/bundle` runtime and module-owned files under
`apps/web/src/lib/i18n/locales/{en,de,ar}/`. Fluent identifiers use hyphens. Tests require
a translation for every registered code in all supported languages. Locale selection follows
an explicit web preference, then supported Accept-Language preferences, not a mutable process-wide
locale, so concurrent users cannot change each other's messages. Unknown failures
receive a generic translation; raw SQL, provider messages, and exception text are
never displayed by the API error renderer.

The current web screens also use module-owned Fluent resources for their labels,
form feedback, and instructions. See [web localization](../../docs/LOCALIZATION.md)
for preference, RTL, and verification conventions. Future screens and emails can
adopt these conventions incrementally. Translate messages, not store names or catalog
content automatically. Structured public error arguments can be added when needed;
permit only declared safe values such as limits or durations, never credentials,
addresses, or raw provider payloads. Preserve Retry-After as protocol metadata.

The separation was informed by the local OpenOM project, inspected read-only; its
implementation and private files were not imported. Fluent usage follows the
[official runtime documentation](https://projectfluent.org/fluent.js/modules/_fluent_bundle.html).
