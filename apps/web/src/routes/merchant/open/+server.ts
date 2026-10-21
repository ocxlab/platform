import { error, redirect } from '@sveltejs/kit';
import { session } from '$lib/server/sessions';
import { text } from '$lib/server/api';
export const GET = ({ cookies, url }) => {
  const current = session(cookies, 'merchant'); const store = url.searchParams.get('store');
  if (!current || !store || !/^[0-9a-f-]{36}$/i.test(store)) error(400, text('merchant-open-required'));
  redirect(303, `/merchant/stores/${current.merchant}/${store}`);
};
