import { error, fail } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { api, message, text } from '$lib/server/api';
import { guestCheckout } from '$lib/server/guest';
import { env } from '$env/dynamic/private';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const guest = guestCheckout(cookies, params.merchant, params.store);
  if (!guest.token()) error(401, text('receipt-browser-required'));
  const result = await api(guest.token()).GET('/api/v1/guest/merchants/{merchant_id}/stores/{store_id}/food/orders/{order_id}', {
    params: { path: { merchant_id: params.merchant, store_id: params.store, order_id: params.order } },
  });
  if (!result.data) error(result.response.status, result.response.status === 404 ? text('receipt-not-available') : message(result.error));
  return { order: result.data, developmentPayments: env.OCP_DEVELOPMENT_PAYMENTS === 'true' };
};
export const actions: Actions = {
  payment: async ({ params, cookies, request }) => {
    const guest = guestCheckout(cookies, params.merchant, params.store);
    if (!guest.token()) return fail(401, { error: text('receipt-browser-required') });
    const form = await request.formData();
    const action = String(form.get('action'));
    if (action !== 'start' && action !== 'succeed' && action !== 'decline' && action !== 'abandon') return fail(400, { error: text('error-common-invalid-request') });
    const result = await api(guest.token()).POST('/api/v1/guest/merchants/{merchant_id}/stores/{store_id}/food/orders/{order_id}/development-payment', {
      params: { path: { merchant_id: params.merchant, store_id: params.store, order_id: params.order }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_revision: Number(form.get('revision')), action },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: text('payment-saved') };
  },
  cancel: async ({ params, cookies, request }) => {
    const guest = guestCheckout(cookies, params.merchant, params.store);
    if (!guest.token()) return fail(401, { error: text('receipt-browser-required') });
    const form = await request.formData();
    const result = await api(guest.token()).POST('/api/v1/guest/merchants/{merchant_id}/stores/{store_id}/food/orders/{order_id}/cancellation', {
      params: { path: { merchant_id: params.merchant, store_id: params.store, order_id: params.order }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_revision: Number(form.get('revision')) },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: text('orders-cancelled') };
  },
};
