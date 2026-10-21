import type { Handle, HandleServerError } from '@sveltejs/kit';
import { direction, isLocale, localeFor } from '$lib/i18n/locales';
import { translate } from '$lib/i18n/messages';
import { forbiddenFormOrigin } from '$lib/server/form-origin';
export const handle: Handle = async ({ event, resolve }) => {
  const selected = event.cookies.get('ocp_locale');
  event.locals.locale = isLocale(selected) ? selected : localeFor(event.request.headers.get('Accept-Language') ?? '');
  if (forbiddenFormOrigin(event.request, event.url.origin)) return new Response('Forbidden', { status: 403, headers: { 'Cache-Control': 'no-store' } });
  // SSR/form results can contain private menu drafts. Never cache across sessions.
  const response = await resolve(event, { transformPageChunk: ({ html }) => html.replace('%ocp.lang%', event.locals.locale).replace('%ocp.dir%', direction(event.locals.locale)) });
  response.headers.set('Cache-Control', 'no-store');
  response.headers.set('X-Content-Type-Options', 'nosniff');
  response.headers.set('Referrer-Policy', 'same-origin');
  response.headers.set('Content-Security-Policy', "frame-ancestors 'none'");
  return response;
};
// Do not dump submitted bootstrap credentials or internal request details to logs.
export const handleError: HandleServerError = ({ event }) => ({ message: translate('error-common-unavailable', event.locals.locale ?? 'en') });
