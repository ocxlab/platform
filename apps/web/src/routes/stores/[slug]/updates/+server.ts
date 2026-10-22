import type { RequestHandler } from './$types';
import { customerStore } from '$lib/server/customer-store';
import { orderUpdates } from '$lib/server/order-updates';

export const GET: RequestHandler = async ({ request, params }) => {
  const store = await customerStore(params.slug);
  return orderUpdates(request, `/api/v1/public/merchants/${store.merchant_id}/stores/${store.id}/offering-updates`);
};
