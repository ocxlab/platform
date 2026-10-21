import createClient from 'openapi-fetch';
import { env } from '$env/dynamic/private';
import type { paths } from '$lib/api/generated';
import { getRequestEvent } from '$app/server';
import { errorText, localeFor } from '$lib/i18n/messages';

// Server-only: one configured internal origin, never a browser-selected proxy URL.
export function api(token?: string) {
  return createClient<paths>({ baseUrl: env.OCP_API_URL ?? 'http://api:8080',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    fetch: (request) => fetch(request, { signal: AbortSignal.timeout(12_000) }),
  });
}
export function message(error: unknown): string {
  let language = '';
  try { language = getRequestEvent().request.headers.get('Accept-Language') ?? ''; } catch { /* No request context in standalone tests. */ }
  return errorText(error, localeFor(language));
}
