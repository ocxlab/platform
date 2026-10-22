import { error, fail, redirect } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { Actions, PageServerLoad } from './$types';
import { api, message, text } from '$lib/server/api';
import { guestCheckout } from '$lib/server/guest';
import { loadReceipt, receiptPath } from '$lib/server/receipt';
import { developmentActions } from '$lib/orders';

export const load: PageServerLoad = async ({ params, cookies }) => {
  if (env.OCP_DEVELOPMENT_PAYMENTS !== 'true') error(404, text('error-common-not-found'));
  const order = await loadReceipt(params, cookies);
  if (order.payment !== 'awaiting_customer' || !developmentActions(order, true).length) redirect(303, receiptPath(params));
  return { order, receipt: receiptPath(params) };
};
export const actions: Actions = {
  default: async ({ params, cookies, request }) => {
    if (env.OCP_DEVELOPMENT_PAYMENTS !== 'true') error(404, text('error-common-not-found'));
    const order = await loadReceipt(params, cookies);
    const guest = guestCheckout(cookies, order.merchant_id, order.store_id);
    if (!guest.token()) return fail(401, { error: text('receipt-browser-required') });
    const form = await request.formData();
    const action = String(form.get('action'));
    if (action !== 'succeed' && action !== 'decline' && action !== 'abandon') return fail(400, { error: text('error-common-invalid-request') });
    const result = await api(guest.token()).POST('/api/v1/guest/merchants/{merchantId}/stores/{storeId}/food/orders/{orderId}/development-payment', {
      params: { path: { merchantId: order.merchant_id, storeId: order.store_id, orderId: order.id }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_revision: Number(form.get('revision')), action },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    redirect(303, receiptPath(params));
  },
};
