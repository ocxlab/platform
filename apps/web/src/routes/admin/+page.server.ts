import { fail } from '@sveltejs/kit';
import { randomUUID } from 'node:crypto';
import type { Actions, PageServerLoad } from './$types';
import { api, message, text } from '$lib/server/api';
import { connect, disconnect, session } from '$lib/server/sessions';
const uuid = /^[0-9a-f-]{36}$/i;
export const load: PageServerLoad = ({ cookies }) => ({ connected: Boolean(session(cookies, 'provider')), request_key:randomUUID() });
export const actions: Actions = {
  connect: async ({ request, cookies }) => {
    const form = await request.formData(); const token = String(form.get('token') ?? '');
    if (!/^ocp_admin_[A-Za-z0-9_-]{43}$/.test(token)) return fail(400, { error: text('admin-invalid-credential') });
    connect(cookies, 'provider', token); return { connected: true };
  },
  disconnect: async ({ cookies }) => { disconnect(cookies, 'provider'); return { disconnected: true }; },
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
