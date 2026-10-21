import createClient from 'openapi-fetch';
import { env } from '$env/dynamic/private';
import type { paths } from '$lib/api/generated';

// Server-only: one configured internal origin, never a browser-selected proxy URL.
export function api(token?: string) {
  return createClient<paths>({ baseUrl: env.OCP_API_URL ?? 'http://api:8080',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    fetch: (request) => fetch(request, { signal: AbortSignal.timeout(12_000) }),
  });
}
export function message(error: unknown): string {
  if (error && typeof error === 'object' && 'detail' in error && typeof error.detail === 'string') return error.detail;
  return 'The service could not complete this request. Refresh before retrying.';
}
