import { fail, redirect } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { api, message, text } from '$lib/server/api';
import { guestCheckout } from '$lib/server/guest';
import { env } from '$env/dynamic/private';
import { loadReceipt, receiptPath } from '$lib/server/receipt';

export const load: PageServerLoad = async ({ params, cookies }) => {
  return { order: await loadReceipt(params, cookies), receipt: receiptPath(params), storefront: `/stores/${encodeURIComponent(params.slug)}`, developmentPayments: env.OCP_DEVELOPMENT_PAYMENTS === 'true' };
};
export const actions: Actions = {
  payment: async ({ params, cookies, request }) => {
    const order = await loadReceipt(params, cookies);
    const guest = guestCheckout(cookies, order.merchant_id, order.store_id);
    if (!guest.token()) return fail(401, { error: text('receipt-browser-required') });
    const form = await request.formData();
    const action = String(form.get('action'));
    if (env.OCP_DEVELOPMENT_PAYMENTS !== 'true' || action !== 'start') return fail(400, { error: text('error-common-invalid-request') });
    const result = await api(guest.token()).POST('/api/v1/guest/merchants/{merchantId}/stores/{storeId}/food/orders/{orderId}/development-payment', {
      params: { path: { merchantId: order.merchant_id, storeId: order.store_id, orderId: order.id }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_revision: Number(form.get('revision')), action },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    redirect(303, `${receiptPath(params)}/checkout`);
  },
  cancel: async ({ params, cookies, request }) => {
    const order = await loadReceipt(params, cookies);
    const guest = guestCheckout(cookies, order.merchant_id, order.store_id);
    if (!guest.token()) return fail(401, { error: text('receipt-browser-required') });
    const form = await request.formData();
    const result = await api(guest.token()).POST('/api/v1/guest/merchants/{merchantId}/stores/{storeId}/food/orders/{orderId}/cancellation', {
      params: { path: { merchantId: order.merchant_id, storeId: order.store_id, orderId: order.id }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_revision: Number(form.get('revision')) },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: text('orders-cancelled') };
  },
};
