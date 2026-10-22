import { error, fail } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { api, message, text } from '$lib/server/api';
import { merchantStore } from '$lib/server/merchant-store';
export const load: PageServerLoad = async ({ params, cookies }) => {
  const current = await merchantStore(cookies, params.slug);
  const result = await api(current.token).GET('/api/v1/merchants/{merchantId}/stores/{storeId}/food/orders/{orderId}', {
    params: { path: { ...current.path, orderId: params.order } },
  });
  if (!result.data) error(result.response.status, message(result.error));
  return { store: current.store, order: result.data };
};
export const actions: Actions = {
  fulfilment: async ({ params, cookies, request }) => {
    const current = await merchantStore(cookies, params.slug);
    const form = await request.formData();
    const action = form.get('action');
    if (action !== 'start_preparation' && action !== 'mark_ready' && action !== 'record_collection') return fail(400, { error: text('error-common-invalid-request') });
    const result = await api(current.token).POST('/api/v1/merchants/{merchantId}/stores/{storeId}/food/orders/{orderId}/fulfilment', {
      params: { path: { ...current.path, orderId: params.order }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_revision: Number(form.get('revision')), action },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: text('orders-saved') };
  },
  decide: async ({ params, cookies, request }) => {
    const current = await merchantStore(cookies, params.slug);
    const form = await request.formData();
    const decision = form.get('decision');
    if (decision !== 'accept' && decision !== 'reject') return fail(400, { error: text('error-common-invalid-request') });
    const result = await api(current.token).POST('/api/v1/merchants/{merchantId}/stores/{storeId}/food/orders/{orderId}/decisions', {
      params: { path: { ...current.path, orderId: params.order }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_revision: Number(form.get('revision')), decision },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: text('orders-saved') };
  },
};
