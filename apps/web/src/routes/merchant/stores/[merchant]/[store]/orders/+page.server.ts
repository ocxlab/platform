import { error, redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { api, message } from '$lib/server/api';
import { session } from '$lib/server/sessions';

export const load: PageServerLoad = async ({ cookies, params, url }) => {
  const current = session(cookies, 'merchant');
  if (!current || current.merchant !== params.merchant) redirect(303, '/merchant');
  const client = api(current.token);
  const path = { merchant_id: params.merchant, store_id: params.store };
  const store = await client.GET('/api/v1/merchants/{merchant_id}/stores/{store_id}', { params: { path } });
  if (!store.data) error(store.response.status, message(store.error));
  const orders = await client.GET('/api/v1/merchants/{merchant_id}/stores/{store_id}/food/orders', {
    params: { path, query: { limit: 50, after: url.searchParams.get('after') ?? undefined } },
  });
  if (!orders.data) error(orders.response.status, message(orders.error));
  return { store: store.data, orders: orders.data };
};
