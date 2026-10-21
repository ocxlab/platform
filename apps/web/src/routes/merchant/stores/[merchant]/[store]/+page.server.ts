import { error, fail, redirect } from '@sveltejs/kit';
import { randomUUID } from 'node:crypto';
import type { PageServerLoad, Actions } from './$types';
import type { components } from '$lib/api/generated';
import { api, message } from '$lib/server/api';
import { session } from '$lib/server/sessions';

function authorized(cookies: Parameters<typeof session>[0], merchant: string) {
  const current = session(cookies, 'merchant');
  if (!current || current.merchant !== merchant) redirect(303, '/merchant');
  return current;
}
export const load: PageServerLoad = async ({ cookies, params }) => {
  const current = authorized(cookies, params.merchant);
  const client = api(current.token); const path = { merchant_id: params.merchant, store_id: params.store };
  const store = await client.GET('/api/v1/merchants/{merchant_id}/stores/{store_id}', { params: { path } });
  if (!store.data) error(store.response.status, message(store.error));
  const draft = await client.GET('/api/v1/merchants/{merchant_id}/stores/{store_id}/offering', { params: { path } });
  if (!draft.data && draft.response.status !== 404) error(draft.response.status, message(draft.error));
  const publication = await client.GET('/api/v1/merchants/{merchant_id}/stores/{store_id}/publication', { params: { path } });
  if (!publication.data) error(publication.response.status, message(publication.error));
  const menu = (draft.data?.document ?? { currency: 'EUR', availability_mode: 'manual', items: [] }) as components['schemas']['FoodMenu'];
  return { store: store.data, draft: draft.data ?? null, menu, publication: publication.data, request_key:randomUUID() };
};
export const actions: Actions = {
  save: async ({ cookies, params, request }) => {
    const current = authorized(cookies, params.merchant);
    const form = await request.formData();
    let document: Record<string, unknown>;
    try { document = JSON.parse(String(form.get('document'))) as Record<string, unknown>; }
    catch { return fail(400, { error: 'The menu could not be read. Please refresh.' }); }
    const result = await api(current.token).PUT('/api/v1/merchants/{merchant_id}/stores/{store_id}/offering', {
      params: { path: { merchant_id: params.merchant, store_id: params.store }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { base_revision: Number(form.get('base')), model: { id: 'ocp.food.menu', version: '1.0.0' }, document },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: 'Draft saved. Customers still see the previously published revision.' };
  },
  publish: async ({ cookies, params, request }) => {
    const current = authorized(cookies, params.merchant); const form = await request.formData();
    const revision = form.get('revision');
    const result = await api(current.token).PUT('/api/v1/merchants/{merchant_id}/stores/{store_id}/publication', {
      params: { path: { merchant_id: params.merchant, store_id: params.store }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_generation: Number(form.get('generation')), revision: revision ? Number(revision) : null },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: revision ? 'Menu published.' : 'Menu withdrawn from public view.' };
  },
};
