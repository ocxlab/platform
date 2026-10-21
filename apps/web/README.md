# Web application

One SvelteKit/TypeScript application supplies three logical surfaces: `/merchant`
manages private menus and publication, `/stores/{merchant}/{store}` shows the public
offering and requests quotes, and `/admin` manages publication visibility. They
share components, not authorization. Business rules remain in the Go API, with no
direct database access from this application.

The merchant workspace lists managed stores by name, including unpublished stores.
Choose a store link to reopen its editor; remembering an identifier is unnecessary.
The same authorized, paginated list operation is available to other API clients.

## Development and verification

Use root `task web:dev` to build and start the application on localhost:18181.
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
`OCP_LOCAL_INSECURE_COOKIES=true` is only for local HTTP Compose. Responses use
no-store; browsers cannot select arbitrary API proxy destinations. Production
login, rate limits, CSP hardening, signup, secrets management, domain routing, and
a broad browser/accessibility matrix remain work before public deployment. This
slice does not create orders or payments.

## Tooling cohort

Exact versions are in `package.json` and `package-lock.json`: Svelte 5, SvelteKit 2,
adapter-node 5, Vite 7, TypeScript 5, openapi-typescript/openapi-fetch, Vitest,
Svelte Testing Library, and Playwright. The Node 24 image is pinned by digest.
TypeScript 5 satisfies this generation cohort; upgrades must be tested together.
The cookie override pins the patched compatible 0.7.2 transitive release.
npm audit is a point-in-time advisory check, not proof of security.

Tool references: [SvelteKit](https://svelte.dev/docs/kit),
[OpenAPI TypeScript](https://openapi-ts.dev/), and
[Playwright](https://playwright.dev/docs/intro). These dependencies are not copied
commerce-standard schemas or evidence of UCP conformance.
