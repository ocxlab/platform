# Web language support

The maintained web UI offers Deutsch, English, and Arabic in a header dropdown.
It covers the current home, merchant/store/menu, guest checkout/receipt, provider
administration, and error screens. Merchant-entered names and offering content
are not translated or rewritten. Arabic is an initial translation, not a native
speaker's linguistic review.

## Preference and ownership

`apps/web/src/lib/i18n/locales.ts` owns the supported language metadata. A valid
`ocp_locale` cookie takes precedence over weighted browser `Accept-Language`
preferences; unsupported preferences fall back to English. The cookie lasts one
year and is scoped to this web host, not an OCN-wide account. It is HttpOnly,
SameSite=Lax, and Secure over HTTPS. The same-origin `/language` form endpoint
persists the choice and returns to a validated local path. It works without
JavaScript and does not change grants, order references, merchant jurisdiction,
currency, or catalog data. Switching languages reloads the page; save unfinished
editor changes first, as with other page navigation.

Locale lives in request locals and a root Svelte context, never mutable global
user state. SSR sets `html` language/direction before display; client navigation
keeps those attributes aligned. Server form feedback uses the same resolved locale
as the screen, including the user's explicit selection over browser preferences.

Fluent resources are grouped by responsibility under
`apps/web/src/lib/i18n/locales/{en,de,ar}/`. Add message keys to all three languages.
Use `useI18n()` in components and `text()` in server operations. Keep stable API
machine codes separate from translations; `message()` maps those codes safely.
Tests check key parity, duplicate keys, referenced keys, and formatting with arguments.

## Mixed text and layout direction

The page uses RTL for Arabic and LTR for German/English. Every language name in
the selector independently declares its own `lang` and `dir` on a `bdi` element:
Deutsch and English remain LTR in an Arabic header; the Arabic name remains RTL
in an English or German header. Do not reverse strings or mirror the whole UI
with transforms. Local SVG flags are decorative visual cues; native names carry
the accessible language choice. Asset provenance and license are in `static/flags/`.

Use logical CSS spacing/alignment (`inline-start`, `inline-end`, `start`), and
isolate merchant names, dish names, choices, and identifiers from surrounding text.
Natural-language inputs use `dir=auto`; credential/identifier and numeric inputs
stay LTR. Currency/date formatting follows the UI locale, while the current EUR
currency and Europe/Berlin business timezone remain explicit domain choices.
Fluent isolates interpolated values. Physical viewport coordinates in tooltip
positioning intentionally remain physical; they are not text alignment rules.

## Verification

Run `task check` and `task test:browser`. Browser checks exercise both directions,
the actual glyph order of Latin names within RTL, independent browser preferences,
cookie persistence, form feedback, no-JavaScript selection, keyboard dismissal,
local flag assets, and mobile bounds. These targeted tests do not replace native
language review, cross-browser coverage, or a full accessibility audit.
