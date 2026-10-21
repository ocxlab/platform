import { fail, redirect } from '@sveltejs/kit';
import { randomUUID } from 'node:crypto';
import type { PageServerLoad } from './$types';
import { api, message } from '$lib/server/api';
import { connect, disconnect, session } from '$lib/server/sessions';
export const load: PageServerLoad = async ({ cookies, url }) => {
  const current = session(cookies, 'merchant');
  const base = { merchant: current?.merchant ?? '', request_key: randomUUID(),
    stores: [] as import('$lib/api/generated').components['schemas']['Store'][],
    next_after: undefined as string | undefined, after: url.searchParams.get('after') ?? '', stores_error: '' };
  if (!current) return base;
  const result = await api(current.token).GET('/api/v1/merchants/{merchant_id}/stores', {
    params: { path: { merchant_id: current.merchant }, query: { limit: 50, ...(base.after ? { after: base.after } : {}) } },
  });
  if (result.response.status === 401) {
    disconnect(cookies, 'merchant');
    return { ...base, merchant: '', stores_error: 'Your development credential has expired or is no longer valid. Connect a credential again.' };
  }
  if (!result.data) return { ...base, stores_error: message(result.error) };
  return { ...base, stores: result.data.items, next_after: result.data.next_after };
};
export const actions = {
  connect: async ({ request, cookies }) => {
    const form = await request.formData();
    const merchant = String(form.get('merchant') ?? ''); const token = String(form.get('token') ?? '');
    if (!/^[0-9a-f-]{36}$/i.test(merchant) || !/^ocp_[A-Za-z0-9_-]{43}$/.test(token)) return fail(400, { error: 'Enter the merchant identifier and its temporary merchant credential.' });
    connect(cookies, 'merchant', token, merchant); return { connected: true };
  },
  disconnect: async ({ cookies }) => { disconnect(cookies, 'merchant'); return { disconnected: true }; },
  create: async ({ request, cookies }) => {
    const current = session(cookies, 'merchant'); if (!current) return fail(401, { error: 'Connect your merchant credential first.' });
    const form = await request.formData();
    const { data, error } = await api(current.token).POST('/api/v1/merchants/{merchant_id}/stores', {
      params: { path: { merchant_id: current.merchant }, header: { 'Idempotency-Key': String(form.get('key')) } },
      body: { display_name: String(form.get('name') ?? '') },
    });
    if (error || !data) return fail(400, { error: message(error) });
    redirect(303, `/merchant/stores/${current.merchant}/${data.id}`);
  },
};
