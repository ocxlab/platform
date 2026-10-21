import { error, redirect } from '@sveltejs/kit';
import { session } from '$lib/server/sessions';
import { api, message, text } from '$lib/server/api';
import { merchantStorePath } from '$lib/merchant-paths';
export const GET = async ({ cookies, url }) => {
  const current = session(cookies, 'merchant'); const store = url.searchParams.get('store');
  if (!current || !store || !/^[0-9a-f-]{36}$/i.test(store)) error(400, text('merchant-open-required'));
  const result = await api(current.token).GET('/api/v1/merchants/{merchant_id}/stores/{store_id}', { params: { path: { merchant_id: current.merchant, store_id: store } } });
  if (!result.data) error(result.response.status, message(result.error));
  redirect(303, merchantStorePath(result.data));
};
