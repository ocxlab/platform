import { error, fail } from '@sveltejs/kit';
import { randomUUID } from 'node:crypto';
import type { PageServerLoad, Actions } from './$types';
import type { components } from '$lib/api/generated';
import { api, message, text } from '$lib/server/api';
import { merchantStore } from '$lib/server/merchant-store';
export const load: PageServerLoad = async ({ cookies, params }) => {
  const current = await merchantStore(cookies, params.slug);
  const client = api(current.token); const path = current.path;
  const draft = await client.GET('/api/v1/merchants/{merchantId}/stores/{storeId}/offering', { params: { path } });
  if (!draft.data && draft.response.status !== 404) error(draft.response.status, message(draft.error));
  const publication = await client.GET('/api/v1/merchants/{merchantId}/stores/{storeId}/publication', { params: { path } });
  if (!publication.data) error(publication.response.status, message(publication.error));
  const menu = (draft.data?.document ?? { currency: 'EUR', availability_mode: 'manual', items: [] }) as components['schemas']['FoodMenu'];
  return { store: current.store, draft: draft.data ?? null, menu, publication: publication.data, request_key:randomUUID() };
};
export const actions: Actions = {
  save: async ({ cookies, params, request }) => {
    const current = await merchantStore(cookies, params.slug);
    const form = await request.formData();
    let document: Record<string, unknown>;
    try { document = JSON.parse(String(form.get('document'))) as Record<string, unknown>; }
    catch { return fail(400, { error: text('merchant-invalid-document') }); }
    const result = await api(current.token).PUT('/api/v1/merchants/{merchantId}/stores/{storeId}/offering', {
      params: { path: current.path, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { base_revision: Number(form.get('base')), model: { id: 'ocp.food.menu', version: '1.0.0' }, document },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: text('merchant-draft-saved') };
  },
  publish: async ({ cookies, params, request }) => {
    const current = await merchantStore(cookies, params.slug); const form = await request.formData();
    const revision = form.get('revision');
    const result = await api(current.token).PUT('/api/v1/merchants/{merchantId}/stores/{storeId}/publication', {
      params: { path: current.path, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_generation: Number(form.get('generation')), revision: revision ? Number(revision) : null },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { success: revision ? text('merchant-published') : text('merchant-withdrawn') };
  },
};
