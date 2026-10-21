import type { PageServerLoad } from './$types';
import { api, message, text } from '$lib/server/api';
import type { components } from '$lib/api/generated';

export const load: PageServerLoad = async ({ url }) => {
  const query = url.searchParams.get('q') ?? '';
  const after = url.searchParams.get('after') ?? '';
  const base = { query, stores: [] as components['schemas']['PublicStore'][], next: '', stores_error: '' };
  try {
    const result = await api().GET('/api/v1/public/stores', {
      params: { query: { limit: 50, ...(query ? { q: query } : {}), ...(after ? { after } : {}) } },
    });
    if (!result.data) return { ...base, stores_error: message(result.error) };
    const next = result.data.next_after ? '/?' + new URLSearchParams({ q: query, after: result.data.next_after }).toString() + '#stores' : '';
    return { ...base, stores: result.data.items, next };
  } catch {
    return { ...base, stores_error: text('ui-stores-unavailable') };
  }
};
