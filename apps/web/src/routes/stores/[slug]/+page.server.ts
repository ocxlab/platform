import { error, fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { components } from '$lib/api/generated';
import { api, message, text } from '$lib/server/api';
import { guestCheckout } from '$lib/server/guest';
import { foodSelection } from '$lib/server/food-selection';
import { customerStore, rememberCustomerStore, customerReceiptScope } from '$lib/server/customer-store';
import { customerStorePath } from '$lib/merchant-paths';
export const load: PageServerLoad = async ({ params, cookies, url }) => {
  const store = await customerStore(params.slug);
  const result = await api().GET('/api/v1/public/merchants/{merchantId}/stores/{storeId}/offering', { params: { path: { merchantId: store.merchant_id, storeId: store.id } } });
  if (!result.data) error(result.response.status, result.response.status === 404 ? text('storefront-no-menu') : message(result.error));
  if (result.data.model.id !== 'ocp.food.menu' || result.data.model.version !== '1.0.0') error(422, text('storefront-unsupported'));
  const guest = guestCheckout(cookies, store.merchant_id, store.id);
  const intent = guest.intent();
  let quote: components['schemas']['FoodQuote'] | undefined;
  if (intent && guest.token()) {
    const saved = await api(guest.token()).GET('/api/v1/guest/merchants/{merchantId}/stores/{storeId}/food/quotes/{quoteId}', { params: { path: { merchantId: store.merchant_id, storeId: store.id, quoteId: intent.quote } } });
    quote = saved.data;
  }
  const ordering = await api().GET('/api/v1/public/merchants/{merchantId}/stores/{storeId}/food/ordering', { params: { path: { merchantId: store.merchant_id, storeId: store.id } } });
  if (!ordering.data) error(ordering.response.status, message(ordering.error));
  const menu = result.data.document as components['schemas']['FoodMenu'];
  const selectedItem = menu.items.find(item => item.id === url.searchParams.get('item') && item.available)?.id ?? '';
  return { offering: result.data, menu, selectedItem, quote, intent, ordering: ordering.data };
};
export const actions: Actions = {
  quote: async ({ params, request, cookies }) => {
    const store = await customerStore(params.slug);
    const body = foodSelection(await request.formData());
    if (!body) return fail(400, { error: text('storefront-select-required') });
    const guest = guestCheckout(cookies, store.merchant_id, store.id);
    const path = { merchantId: store.merchant_id, storeId: store.id };
    let token = guest.token();
    if (!token) {
      const session = await api().POST('/api/v1/public/merchants/{merchantId}/stores/{storeId}/guest-sessions', { params: { path }, body: {} });
      if (!session.data) return fail(session.response.status, { error: message(session.error) });
      token = session.data.access_token;
      guest.setToken(token, session.data.expires_at);
    }
    const result = await api(token).POST('/api/v1/guest/merchants/{merchantId}/stores/{storeId}/food/quotes', { params: { path }, body });
    if (result.response.status === 401) guest.clear();
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    const intent = guest.setIntent(result.data.id);
    rememberCustomerStore(cookies, store);
    if (request.headers.get('x-ocp-basket-preview') === 'true') return { quote: result.data, intent };
    // Post/redirect/get retains the quote and placement key across refresh/lost
    // placement responses. The cookie stores references, never authoritative prices.
    redirect(303, `${customerStorePath(store)}#selection-quote`);
  },
  place: async ({ params, cookies, request }) => {
    // Replay remains possible after withdrawal: the routing cookie is only a
    // hint; placement authority and replay are still checked by the guest API.
    const scope = customerReceiptScope(cookies, params.slug);
    const guest = guestCheckout(cookies, scope.merchant, scope.store);
    const intent = guest.intent();
    const form = await request.formData();
    if (!guest.token() || !intent) return fail(401, { error: text('storefront-session-ended') });
    if (form.get('quote') !== intent.quote || form.get('key') !== intent.key) return fail(409, { error: text('storefront-intent-changed') });
    let result;
    try {
      result = await api(guest.token()).POST('/api/v1/guest/merchants/{merchantId}/stores/{storeId}/food/orders', {
        params: { path: { merchantId: scope.merchant, storeId: scope.store }, header: { 'Idempotency-Key': intent.key } },
        body: { quote_id: intent.quote, fulfilment: 'pickup' },
      });
    } catch {
      return fail(503, { error: text('storefront-retry') });
    }
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    let order = result.data;
    if (!order.reference) {
      const current = await api(guest.token()).GET('/api/v1/guest/merchants/{merchantId}/stores/{storeId}/food/orders/{orderId}', { params: { path: { merchantId: scope.merchant, storeId: scope.store, orderId: order.id } } });
      if (!current.data) return fail(current.response.status, { error: message(current.error) });
      order = current.data;
    }
    if (!order.reference) return fail(503, { error: text('storefront-retry') });
    redirect(303, `/stores/${encodeURIComponent(params.slug)}/orders/${order.reference}`);
  },
};
