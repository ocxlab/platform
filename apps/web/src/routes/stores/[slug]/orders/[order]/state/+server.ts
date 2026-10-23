import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { loadReceipt } from '$lib/server/receipt';

// Same HttpOnly guest authority as the SSR receipt; never a public order proxy.
export const GET: RequestHandler = async ({ params, cookies, request, setHeaders }) => {
  setHeaders({ 'cache-control': 'private, no-store', vary: 'Cookie' });
  return json(await loadReceipt(params, cookies, request.signal));
};
