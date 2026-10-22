import type { RequestHandler } from './$types';
import { merchantStore } from '$lib/server/merchant-store';
import { orderUpdates } from '$lib/server/order-updates';

export const GET: RequestHandler = async ({ request, params, cookies }) => {
  const current = await merchantStore(cookies, params.slug);
  return orderUpdates(request, `/api/v1/merchants/${current.merchant}/stores/${current.store.id}/food/order-updates`, current.token);
};
