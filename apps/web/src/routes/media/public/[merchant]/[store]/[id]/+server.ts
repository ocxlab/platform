import type { RequestHandler } from './$types';
import { imageRedirect } from '$lib/server/media';
export const GET: RequestHandler = async ({ params, setHeaders }) => {
  setHeaders({ 'cache-control': 'no-store', 'referrer-policy': 'no-referrer' });
  return imageRedirect(params.merchant, params.store, params.id);
};
