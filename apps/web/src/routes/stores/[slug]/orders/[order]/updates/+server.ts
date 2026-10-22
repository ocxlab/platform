import type { RequestHandler } from './$types';
import { loadReceipt } from '$lib/server/receipt';
import { customerReceiptScope } from '$lib/server/customer-store';
import { guestCheckout } from '$lib/server/guest';
import { orderUpdates } from '$lib/server/order-updates';

export const GET: RequestHandler = async ({ request, params, cookies }) => {
  const order = await loadReceipt(params, cookies);
  const scope = customerReceiptScope(cookies, params.slug);
  const token = guestCheckout(cookies, scope.merchant, scope.store).token()!;
  return orderUpdates(request, `/api/v1/guest/merchants/${scope.merchant}/stores/${scope.store}/food/order-updates/${order.id}`, token);
};
