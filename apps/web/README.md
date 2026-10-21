# Web application

One SvelteKit/TypeScript application supplies three logical surfaces: `/merchant`
manages private menus and publication, `/stores/{merchant}/{store}` shows the public
offering and requests quotes, and `/admin` manages publication visibility. They
share components, not authorization. Business rules remain in the Go API, with no
direct database access from this application.

The merchant workspace lists managed stores in a table, including unpublished stores.
Columns show linked store names, latest draft menu-item counts, draft revisions,
and last draft-save times (or creation before the first save), in Europe/Berlin.
Choose a store link to reopen its editor; remembering an identifier is unnecessary.
The same authorized, paginated list operation is available to other API clients.

## Action buttons

`ActionButton.svelte` provides Lucide icons, explicit accessible names, and
tooltips on pointer hover or keyboard focus. Escape dismisses a tooltip; it stays
available while the pointer is over it. Familiar deletion actions can be icon-only;
less obvious actions retain visible labels. Buttons without icons also retain
their labels. Icons are decorative for assistive technology, and native form
submission, disabled states, and event handlers are preserved.

The exact `@lucide/svelte` version is pinned in package.json and its lockfile.
Use direct per-icon imports following the [Lucide Svelte guide](https://lucide.dev/guide/svelte/getting-started).
Add future dependencies through `task web:add PACKAGE=name` inside Docker.

## Development and verification

Use root `task web:dev` to start local Svelte/Vite hot reload on localhost:18181,
with Node.js 22.12+ or 24 and pnpm 11.20.0. The backend/database stay in Docker.
`task web:container` retains the fully containerized built alternative; stop local
Vite before using it. Both modes use the same pnpm lockfile. Dependency install
scripts are disabled. The pnpm workspace file owns overrides and build policy.
Use `task dev:credential` for merchant bootstrap and `task dev:admin-credential`
for the separate admin bootstrap. `task web:check` checks generated API drift,
Svelte/TypeScript, focused Vitest tests, and the runtime image. `task test:browser`
runs the joined Playwright journey with a disposable database and synthetic
short-lived credentials. Do not run it concurrently with `task test:integration`:
both use the disposable `test-db` service. Persistent development data is separate.

The browser journey covers private drafts, exact publication, required extras,
server pricing, separate admin suspension/restoration, session cookie attributes,
credential absence from page data/storage, and a cross-origin mutation rejection.
It is a Chromium integration check, not independent interoperability, a full
accessibility audit, or production authentication certification. Test traces,
screenshots, and videos are disabled to avoid retaining bootstrap credentials.

## Source ownership

- `src/routes/`: server loaders/actions and UI; actions call supported APIs rather
  than implementing an alternative business lifecycle.
- `src/lib/api/generated.d.ts`: types generated from modular OpenAPI; change the
  source contracts and run `task web:generate`, never edit generated types.
- `src/lib/server/`: fixed-origin typed API access and server-held session handles.
- `src/lib/components/`: reusable notices and the Food-specific menu editor.
- `tests/`: browser journeys; focused tests live beside their implementation.
- `scripts/`: runner receives disposable credentials over stdin, not a tracked
  file or echoed command, and passes them only to its test child.

## Security and deployment boundary

Credential entry is development-only. The browser receives an opaque one-hour
HttpOnly/SameSite=Strict handle; bearer credentials stay in server memory. Merchant
and provider audiences are checked separately. Restarting the web process signs
users out; multiple replicas need a production identity/session design rather
than this bootstrap map. Backend grants are checked on every action.

`OCP_API_URL` is a trusted server-only origin. `ORIGIN` must match the browser
origin for SvelteKit CSRF checks. Cookies are Secure by default;
`OCP_LOCAL_INSECURE_COOKIES=true` is only for local HTTP development. Responses use
no-store; browsers cannot select arbitrary API proxy destinations. Production
login, rate limits, CSP hardening, signup, secrets management, domain routing, and
a broad browser/accessibility matrix remain work before public deployment. This
slice supports guest pickup-order placement, but not real payments or kitchen work.

## Tooling cohort

Exact versions are in `package.json` and `pnpm-lock.yaml`: Svelte 5, SvelteKit 2,
adapter-node 5, Vite 7, TypeScript 5, openapi-typescript/openapi-fetch, Vitest,
Svelte Testing Library, and Playwright. The Node 24 image is pinned by digest.
TypeScript 5 satisfies this generation cohort; upgrades must be tested together.
The cookie override pins the patched compatible 0.7.2 transitive release.
An advisory audit is a point-in-time check, not proof of security.

Tool references: [SvelteKit](https://svelte.dev/docs/kit),
[OpenAPI TypeScript](https://openapi-ts.dev/), and
[Playwright](https://playwright.dev/docs/intro). These dependencies are not copied
commerce-standard schemas or evidence of UCP conformance.
