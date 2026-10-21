const cursor = /^[0-9a-f]{64}:[0-9]{1,19}$/;

// Keep filters and bounded backward history in links, not a JS-only table cache.
// API continuations remain positions, never credentials or snapshot guarantees.
export function adminNavigation(url: URL) {
  const tab = url.searchParams.get('tab') === 'orders' ? 'orders' : 'stores';
  const q = (url.searchParams.get('q') ?? '').trim();
  const status = url.searchParams.get('status') ?? '';
  const limit = Number(url.searchParams.get('limit') ?? '25');
  const after = url.searchParams.get('after') ?? '';
  const back = url.searchParams.getAll('back').slice(-20);
  const states = tab === 'stores' ? ['unpublished', 'published', 'suspended'] : ['pending', 'accepted', 'rejected', 'cancelled'];
  const invalid = q.length > 160 || ![10, 25, 50].includes(limit) || Boolean(status && !states.includes(status)) || Boolean(after && !cursor.test(after)) || back.some(value => value !== '' && !cursor.test(value));
  const base = new URLSearchParams({ tab, q, status, limit: String(limit) });
  const link = (position: string, history: string[]) => {
    const params = new URLSearchParams(base);
    if (position) params.set('after', position);
    for (const value of history.slice(-20)) params.append('back', value);
    return `/admin?${params}`;
  };
  const current = link(after, back);
  return { tab, q, status, limit, after, invalid, query: current.split('?')[1], first: link('', []),
    previous: after && back.length ? link(back.at(-1)!, back.slice(0, -1)) : undefined,
    next: (position: string) => link(position, [...back, after]) };
}
