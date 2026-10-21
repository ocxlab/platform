import { error, fail, redirect } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { api, message, text } from '$lib/server/api';
import { session } from '$lib/server/sessions';

function authorized(cookies: Parameters<typeof session>[0], merchant: string) {
  const current = session(cookies, 'merchant');
  if (!current || current.merchant !== merchant) redirect(303, '/merchant');
  return current;
}
export const load: PageServerLoad = async ({ params, cookies }) => {
  const current = authorized(cookies, params.merchant);
  const result = await api(current.token).GET('/api/v1/merchants/{merchant_id}/stores/{store_id}/food/orders/{order_id}', {
    params: { path: { merchant_id: params.merchant, store_id: params.store, order_id: params.order } },
  });
  if (!result.data) error(result.response.status, message(result.error));
  return { order: result.data };
};
export const actions: Actions = {
  fulfilment: async ({ params, cookies, request }) => {
    const current = authorized(cookies, params.merchant);
    const form = await request.formData();
    const action = form.get('action');
    if (action !== 'start_preparation' && action !== 'mark_ready' && action !== 'record_collection') return fail(400, { error: text('error-common-invalid-request') });
    const result = await api(current.token).POST('/api/v1/merchants/{merchant_id}/stores/{store_id}/food/orders/{order_id}/fulfilment', {
      params: { path: { merchant_id: params.merchant, store_id: params.store, order_id: params.order }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_revision: Number(form.get('revision')), action },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: text('orders-saved') };
  },
  decide: async ({ params, cookies, request }) => {
    const current = authorized(cookies, params.merchant);
    const form = await request.formData();
    const decision = form.get('decision');
    if (decision !== 'accept' && decision !== 'reject') return fail(400, { error: text('error-common-invalid-request') });
    const result = await api(current.token).POST('/api/v1/merchants/{merchant_id}/stores/{store_id}/food/orders/{order_id}/decisions', {
      params: { path: { merchant_id: params.merchant, store_id: params.store, order_id: params.order }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_revision: Number(form.get('revision')), decision },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: text('orders-saved') };
  },
};
