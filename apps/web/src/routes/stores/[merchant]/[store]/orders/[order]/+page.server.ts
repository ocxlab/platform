import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { api, message } from '$lib/server/api';
import { guestCheckout } from '$lib/server/guest';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const guest = guestCheckout(cookies, params.merchant, params.store);
  if (!guest.token()) error(401, 'Open this order in the browser used at checkout. Its identifier alone does not grant access.');
  const result = await api(guest.token()).GET('/api/v1/guest/merchants/{merchant_id}/stores/{store_id}/food/orders/{order_id}', {
    params: { path: { merchant_id: params.merchant, store_id: params.store, order_id: params.order } },
  });
  if (!result.data) error(result.response.status, result.response.status === 404 ? 'This order is not available to this checkout session.' : message(result.error));
  return { order: result.data };
};
