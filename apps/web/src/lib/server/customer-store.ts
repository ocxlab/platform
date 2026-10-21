import { error, type Cookies } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { storeHandleFromSlug } from '$lib/merchant-paths';
import { api, message, text } from './api';

export async function customerStore(slug: string) {
  const handle = storeHandleFromSlug(slug);
  if (!handle) error(404, text('storefront-no-menu'));
  const result = await api().GET('/api/v1/public/stores/{store_handle}', { params: { path: { store_handle: handle } } });
  if (!result.data) error(result.response.status, result.response.status === 404 ? text('storefront-no-menu') : message(result.error));
  return result.data;
}

// Routing hints only, never authority. The API authenticates the scoped guest
// and authorizes every order read/action. Keep receipts independent of publication.
export function rememberCustomerStore(cookies: Cookies, store: { handle?: string; merchant_id: string; id: string }) {
  if (!store.handle) throw new Error('Current store handle required');
  cookies.set(`ocp_route_${store.handle}`, `${store.merchant_id}.${store.id}`, {
    path: '/stores', httpOnly: true, sameSite: 'strict', secure: env.OCP_LOCAL_INSECURE_COOKIES !== 'true', maxAge: 604800,
  });
}

export function customerReceiptScope(cookies: Cookies, slug: string) {
  const handle = storeHandleFromSlug(slug);
  const ids = handle && cookies.get(`ocp_route_${handle}`)?.split('.');
  if (!ids || ids.length !== 2 || ids.some(id => !/^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/.test(id))) {
    error(401, text('receipt-browser-required'));
  }
  return { merchant: ids[0], store: ids[1] };
}
