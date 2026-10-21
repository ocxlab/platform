import { fail } from '@sveltejs/kit';
import { randomUUID } from 'node:crypto';
import type { Actions, PageServerLoad } from './$types';
import { api, message, text } from '$lib/server/api';
import { connect, disconnect, session } from '$lib/server/sessions';
import type { components } from '$lib/api/generated';
import { adminNavigation } from '$lib/server/admin-navigation';
const uuid = /^[0-9a-f-]{36}$/i;
export const load: PageServerLoad = async ({ cookies, url }) => {
  const current = session(cookies, 'provider');
  const navigation = adminNavigation(url);
  let stores: components['schemas']['AdminStorePage']['items'] = [];
  let orders: components['schemas']['AdminOrderPage']['items'] = [];
  let list_error = navigation.invalid ? text('error-common-invalid-request') : undefined;
  let next: string | undefined;
  if (current && !list_error) {
    try {
      const params = { q: navigation.q || undefined, limit: navigation.limit, after: navigation.after || undefined };
      if (navigation.tab === 'orders') {
        const result = await api(current.token).GET('/api/v1/admin/food/orders', { params: { query: { ...params, status: navigation.status as 'pending' | 'accepted' | 'rejected' | 'cancelled' || undefined } } });
        if (result.data) { orders = result.data.items; next = result.data.next_after; }
        else list_error = message(result.error);
      } else {
        const result = await api(current.token).GET('/api/v1/admin/stores', { params: { query: { ...params, status: navigation.status as 'unpublished' | 'published' | 'suspended' || undefined } } });
        if (result.data) { stores = result.data.items; next = result.data.next_after; }
        else list_error = message(result.error);
      }
    } catch {
      list_error = text('error-common-unavailable');
    }
  }
  return { connected: Boolean(current), request_key: randomUUID(), stores, orders, list_error,
    tab: navigation.tab, q: navigation.q, status: navigation.status, limit: navigation.limit,
    first_url: navigation.first, previous_url: navigation.previous,
    next_url: next ? navigation.next(next) : undefined, list_query: navigation.query,
    paged: Boolean(navigation.after), refresh_url: `/admin?${navigation.query}` };
};
export const actions: Actions = {
  connect: async ({ request, cookies }) => {
    const form = await request.formData(); const token = String(form.get('token') ?? '');
    if (!/^ocp_admin_[A-Za-z0-9_-]{43}$/.test(token)) return fail(400, { error: text('admin-invalid-credential') });
    connect(cookies, 'provider', token); return { connected: true };
  },
  disconnect: async ({ cookies }) => { disconnect(cookies, 'provider'); return { disconnected: true }; },
  inspectOrder: async ({ request, cookies }) => {
    const current = session(cookies, 'provider'); if (!current) return fail(401, { error: text('admin-connect-first') });
    const form = await request.formData();
    const path = { merchant_id: String(form.get('merchant') ?? ''), store_id: String(form.get('store') ?? ''), order_id: String(form.get('order') ?? '') };
    if (!Object.values(path).every((value) => uuid.test(value))) return fail(400, { error: text('error-common-invalid-request') });
    const result = await api(current.token).GET('/api/v1/admin/merchants/{merchant_id}/stores/{store_id}/food/orders/{order_id}', { params: { path } });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { order: result.data };
  },
  inspect: async ({ request, cookies }) => {
    const current = session(cookies, 'provider'); if (!current) return fail(401, { error: text('admin-connect-first') });
    const form = await request.formData(); const path = { merchant_id: String(form.get('merchant') ?? ''), store_id: String(form.get('store') ?? '') };
    if (!uuid.test(path.merchant_id) || !uuid.test(path.store_id)) return fail(400, { error: text('admin-identifiers-required') });
    const result = await api(current.token).GET('/api/v1/admin/merchants/{merchant_id}/stores/{store_id}/publication', { params: { path } });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { publication: result.data };
  },
  suspend: async ({ request, cookies }) => {
    const current = session(cookies, 'provider'); if (!current) return fail(401, { error: text('admin-connect-first') });
    const form = await request.formData();
    const result = await api(current.token).PUT('/api/v1/admin/merchants/{merchant_id}/stores/{store_id}/publication', {
      params: { path: { merchant_id: String(form.get('merchant')), store_id: String(form.get('store')) }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { expected_generation: Number(form.get('generation')), suspended: form.get('suspended') === 'true', reason: String(form.get('reason') ?? '') },
    });
    if (!result.data) return fail(result.response.status, { error: message(result.error) });
    return { publication: result.data, success: result.data.suspended ? text('admin-suspend-success') : text('admin-restore-success') };
  },
};
