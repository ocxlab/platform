import { error, fail } from '@sveltejs/kit';
import { randomUUID } from 'node:crypto';
import type { Actions, PageServerLoad } from './$types';
import { api, message } from '$lib/server/api';
import { merchantStore } from '$lib/server/merchant-store';

export const load: PageServerLoad = async ({ cookies, params, url }) => {
  const current = await merchantStore(cookies, params.slug);
  const client = api(current.token);
  const path = current.path;
  const orders = await client.GET('/api/v1/merchants/{merchantId}/stores/{storeId}/food/orders', {
    params: { path, query: { limit: 50, after: url.searchParams.get('after') ?? undefined } },
  });
  if (!orders.data) error(orders.response.status, message(orders.error));
  const ordering = await client.GET('/api/v1/merchants/{merchantId}/stores/{storeId}/food/ordering', { params: { path } });
  if (!ordering.data) error(ordering.response.status, message(ordering.error));
  return { store: current.store, orders: orders.data, ordering: ordering.data, settingsKey: randomUUID() };
};

export const actions: Actions = {
  ordering: async ({ cookies, params, request }) => {
    const current = await merchantStore(cookies, params.slug);
    const form = await request.formData();
    const revision = Number(form.get('revision'));
    const mode = form.get('mode');
    const key = form.get('key');
    if (!form.has('revision') || !Number.isSafeInteger(revision) || revision < 0 || typeof key !== 'string' || key.length < 16 || key.length > 128 || (mode !== 'manual' && mode !== 'automatic')) {
      return fail(400, { error: message({ code: 'invalid_request' }) });
    }
    const result = await api(current.token).PUT('/api/v1/merchants/{merchantId}/stores/{storeId}/food/ordering', {
      params: { path: current.path, header: { 'Idempotency-Key': key } },
      body: { expected_revision: revision, accepting_orders: form.get('accepting') === 'on', acceptance_mode: mode },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { saved: true };
  },
};
