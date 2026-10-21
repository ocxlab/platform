import { error, redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { isLocale } from '$lib/i18n/locales';

export const POST: RequestHandler = async ({ request, cookies, url }) => {
  // Endpoint POSTs are not SvelteKit form actions; enforce same-origin explicitly.
  if (request.headers.get('origin') !== url.origin) error(403, 'Forbidden');
  const form = await request.formData();
  const locale = form.get('locale');
  if (!isLocale(locale)) error(400, 'Unsupported language');
  const returnTo = form.get('return_to');
  const target = typeof returnTo === 'string' && returnTo.startsWith('/') && !returnTo.startsWith('//') && !/[\\\r\n]/.test(returnTo) ? returnTo : '/';
  cookies.set('ocp_locale', locale, { path: '/', httpOnly: true, sameSite: 'lax', secure: url.protocol === 'https:', maxAge: 365 * 24 * 60 * 60 });
  redirect(303, target);
};
