import type { RequestHandler } from './$types';
import { merchantStore } from '$lib/server/merchant-store';
import { imageRedirect } from '$lib/server/media';
export const GET: RequestHandler = async ({ params, cookies, setHeaders }) => {
  const current = await merchantStore(cookies, params.slug);
  setHeaders({ 'cache-control': 'no-store', 'referrer-policy': 'no-referrer' });
  return imageRedirect(current.path.merchant_id, current.path.store_id, params.id, current.token);
};
