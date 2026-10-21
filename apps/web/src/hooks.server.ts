import type { Handle, HandleServerError } from '@sveltejs/kit';
export const handle: Handle = async ({ event, resolve }) => {
  // SSR/form results can contain private menu drafts. Never cache across sessions.
  const response = await resolve(event);
  response.headers.set('Cache-Control', 'no-store');
  response.headers.set('X-Content-Type-Options', 'nosniff');
  response.headers.set('Referrer-Policy', 'same-origin');
  response.headers.set('Content-Security-Policy', "frame-ancestors 'none'");
  return response;
};
// Do not dump submitted bootstrap credentials or internal request details to logs.
export const handleError: HandleServerError = () => ({ message: 'The web service could not complete this request.' });
