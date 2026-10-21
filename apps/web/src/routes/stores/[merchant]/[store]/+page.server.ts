import { error, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { components } from '$lib/api/generated';
import { api, message } from '$lib/server/api';
export const load: PageServerLoad = async ({ params }) => {
  const result = await api().GET('/api/v1/public/merchants/{merchant_id}/stores/{store_id}/offering', { params: { path: { merchant_id: params.merchant, store_id: params.store } } });
  if (!result.data) error(result.response.status, result.response.status === 404 ? 'This store has no public menu available.' : message(result.error));
  if (result.data.model.id !== 'ocp.food.menu' || result.data.model.version !== '1.0.0') error(422, 'This storefront does not support this offering model yet.');
  return { offering: result.data, menu: result.data.document as components['schemas']['FoodMenu'] };
};
export const actions: Actions = {
  quote: async ({ params, request }) => {
    const form = await request.formData(); let body: components['schemas']['FoodSelectionRequest'];
    try { body = JSON.parse(String(form.get('selection'))) as typeof body; } catch { return fail(400, { error: 'Select an item, variant, and any required options.' }); }
    const result = await api().POST('/api/v1/public/merchants/{merchant_id}/stores/{store_id}/food/quotes', { params: { path: { merchant_id: params.merchant, store_id: params.store } }, body });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { quote: result.data };
  },
};
