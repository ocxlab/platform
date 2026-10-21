import { error } from '@sveltejs/kit';
import type { Cookies } from '@sveltejs/kit';
import { api, message, text } from './api';
import { guestCheckout } from './guest';

export type ReceiptRoute = { merchant: string; store: string; order: string };
export function receiptPath(params: ReceiptRoute) {
  return `/stores/${params.merchant}/${params.store}/orders/${params.order}`;
}
export async function loadReceipt(params: ReceiptRoute, cookies: Cookies) {
  const token = guestCheckout(cookies, params.merchant, params.store).token();
  if (!token) error(401, text('receipt-browser-required'));
  const result = await api(token).GET('/api/v1/guest/merchants/{merchant_id}/stores/{store_id}/food/orders/{order_id}', {
    params: { path: { merchant_id: params.merchant, store_id: params.store, order_id: params.order } },
  });
  if (!result.data) error(result.response.status, result.response.status === 404 ? text('receipt-not-available') : message(result.error));
  return result.data;
}
