import { error } from '@sveltejs/kit';
import type { Cookies } from '@sveltejs/kit';
import { api, message, text } from './api';
import { guestCheckout } from './guest';
import { customerReceiptScope } from './customer-store';

export type ReceiptRoute = { slug: string; order: string };
export function receiptPath(params: ReceiptRoute) {
  return `/stores/${encodeURIComponent(params.slug)}/orders/${params.order}`;
}
export async function loadReceipt(params: ReceiptRoute, cookies: Cookies, signal?: AbortSignal) {
  const scope = customerReceiptScope(cookies, params.slug);
  const token = guestCheckout(cookies, scope.merchant, scope.store).token();
  if (!token) error(401, text('receipt-browser-required'));
  if (!/^O-[0-9]{6,19}$/.test(params.order)) error(404, text('receipt-not-available'));
  const result = await api(token, signal).GET('/api/v1/guest/merchants/{merchantId}/stores/{storeId}/food/orders/by-reference/{orderReference}', {
    params: { path: { merchantId: scope.merchant, storeId: scope.store, orderReference: params.order } },
  });
  if (!result.data) error(result.response.status, result.response.status === 404 ? text('receipt-not-available') : message(result.error));
  return result.data;
}
