import createClient from 'openapi-fetch';
import { env } from '$env/dynamic/private';
import type { paths } from '$lib/api/generated';
import { getRequestEvent } from '$app/server';
import { errorText, translate } from '$lib/i18n/messages';
import type { Locale } from '$lib/i18n/locales';

// Server-only: one configured internal origin, never a browser-selected proxy URL.
export function api(token?: string) {
  return createClient<paths>({ baseUrl: env.OCP_API_URL ?? 'http://api:8080',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    fetch: (request) => fetch(request, { signal: AbortSignal.timeout(12_000) }),
  });
}
export function message(error: unknown): string {
  return errorText(error, requestLocale());
}
function requestLocale(): Locale {
  try { return getRequestEvent().locals.locale ?? 'en'; } catch { return 'en'; }
}
export function text(key: string, args?: Parameters<typeof translate>[2]): string { return translate(key, requestLocale(), args); }
