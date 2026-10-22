import { error, redirect, type Cookies } from '@sveltejs/kit';
import { api, message, text } from './api';
import { session } from './sessions';
import { storeHandleFromSlug } from '$lib/merchant-paths';

// Resolve through the same tenant-authorized API on every read and mutation.
// A readable URL, or a guessed handle, never grants access to another merchant.
export async function merchantStore(cookies: Cookies, slug: string) {
  const current = session(cookies, 'merchant');
  if (!current) redirect(303, '/merchant');
  const handle = storeHandleFromSlug(slug);
  if (!handle) error(404, text('error-common-not-found'));
  const result = await api(current.token).GET('/api/v1/merchants/{merchantId}/stores', {
    params: { path: { merchantId: current.merchant }, query: { handle, limit: 1 } },
  });
  if (!result.data) error(result.response.status, message(result.error));
  const store = result.data.items[0];
  if (!store) error(404, text('error-common-not-found'));
  return { ...current, store, path: { merchantId: current.merchant, storeId: store.id } };
}
