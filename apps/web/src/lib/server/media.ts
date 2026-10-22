import { error, redirect } from '@sveltejs/kit';
import { api, message } from './api';

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
export async function imageRedirect(merchant: string, store: string, id: string, token?: string): Promise<never> {
  if (![merchant, store, id].every(value => uuid.test(value))) error(404);
  const path = { merchantId: merchant, storeId: store, mediaId: id };
  const result = token
    ? await api(token).GET('/api/v1/merchants/{merchantId}/stores/{storeId}/media/{mediaId}/image', { params: { path }, redirect: 'manual', parseAs: 'text' })
    : await api().GET('/api/v1/public/merchants/{merchantId}/stores/{storeId}/media/{mediaId}/image', { params: { path }, redirect: 'manual', parseAs: 'text' });
  const location = result.response.headers.get('location');
  if (result.response.status !== 307 || !location) error(result.response.status >= 400 ? result.response.status : 503, message(result.error));
  // This location comes only from the configured API storage signer, never a
  // browser-provided URL. Redirect the client; do not proxy image bytes.
  redirect(307, location);
}
