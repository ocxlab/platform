import { error, fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { components } from '$lib/api/generated';
import { api, message } from '$lib/server/api';
import { guestCheckout } from '$lib/server/guest';
export const load: PageServerLoad = async ({ params, cookies }) => {
  const result = await api().GET('/api/v1/public/merchants/{merchant_id}/stores/{store_id}/offering', { params: { path: { merchant_id: params.merchant, store_id: params.store } } });
  if (!result.data) error(result.response.status, result.response.status === 404 ? 'This store has no public menu available.' : message(result.error));
  if (result.data.model.id !== 'ocp.food.menu' || result.data.model.version !== '1.0.0') error(422, 'This storefront does not support this offering model yet.');
  const guest = guestCheckout(cookies, params.merchant, params.store);
  const intent = guest.intent();
  let quote: components['schemas']['FoodQuote'] | undefined;
  if (intent && guest.token()) {
    const saved = await api(guest.token()).GET('/api/v1/guest/merchants/{merchant_id}/stores/{store_id}/food/quotes/{quote_id}', { params: { path: { merchant_id: params.merchant, store_id: params.store, quote_id: intent.quote } } });
    quote = saved.data;
  }
  return { offering: result.data, menu: result.data.document as components['schemas']['FoodMenu'], quote, intent };
};
export const actions: Actions = {
  quote: async ({ params, request, cookies }) => {
    const form = await request.formData(); let body: components['schemas']['FoodSelectionRequest'];
    try { body = JSON.parse(String(form.get('selection'))) as typeof body; } catch { return fail(400, { error: 'Select an item, variant, and any required options.' }); }
    const guest = guestCheckout(cookies, params.merchant, params.store);
    const path = { merchant_id: params.merchant, store_id: params.store };
    let token = guest.token();
    if (!token) {
      const session = await api().POST('/api/v1/public/merchants/{merchant_id}/stores/{store_id}/guest-sessions', { params: { path }, body: {} });
      if (!session.data) return fail(session.response.status, { error: message(session.error) });
      token = session.data.access_token;
      guest.setToken(token, session.data.expires_at);
    }
    const result = await api(token).POST('/api/v1/guest/merchants/{merchant_id}/stores/{store_id}/food/quotes', { params: { path }, body });
    if (result.response.status === 401) guest.clear();
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    guest.setIntent(result.data.id);
    // Post/redirect/get retains the quote and placement key across refresh/lost
    // placement responses. The cookie stores references, never authoritative prices.
    redirect(303, `/stores/${params.merchant}/${params.store}`);
  },
  place: async ({ params, cookies, request }) => {
    const guest = guestCheckout(cookies, params.merchant, params.store);
    const intent = guest.intent();
    const form = await request.formData();
    if (!guest.token() || !intent) return fail(401, { error: 'Your checkout session has ended. Check your selection price again.' });
    if (form.get('quote') !== intent.quote || form.get('key') !== intent.key) return fail(409, { error: 'This checkout has changed in another tab. Refresh and review the selection before placing it.' });
    let result;
    try {
      result = await api(guest.token()).POST('/api/v1/guest/merchants/{merchant_id}/stores/{store_id}/food/orders', {
        params: { path: { merchant_id: params.merchant, store_id: params.store }, header: { 'Idempotency-Key': intent.key } },
        body: { quote_id: intent.quote, fulfilment: 'pickup' },
      });
    } catch {
      return fail(503, { error: 'The order result could not be retrieved. Retry this placement to recover its result without creating another order.' });
    }
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    redirect(303, `/stores/${params.merchant}/${params.store}/orders/${result.data.id}`);
  },
};
