import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { api, message } from '$lib/server/api';
import { merchantStore } from '$lib/server/merchant-store';

export const load: PageServerLoad = async ({ cookies, params, url }) => {
  const current = await merchantStore(cookies, params.slug);
  const client = api(current.token);
  const path = current.path;
  const orders = await client.GET('/api/v1/merchants/{merchant_id}/stores/{store_id}/food/orders', {
    params: { path, query: { limit: 50, after: url.searchParams.get('after') ?? undefined } },
  });
  if (!orders.data) error(orders.response.status, message(orders.error));
  return { store: current.store, orders: orders.data };
};
