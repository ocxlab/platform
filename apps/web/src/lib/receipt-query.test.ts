import { describe, expect, it, vi } from 'vitest';
import { QueryClient, QueryObserver } from '@tanstack/svelte-query';
import { receiptQuery, type ReceiptOrder } from './receipt-query';

const initial = { id: 'order-a', revision: 1 } as ReceiptOrder;

describe('private receipt query', () => {
  it('seeds SSR data without a fetch, then explicitly refreshes from the same-origin endpoint', async () => {
    const read = vi.fn().mockResolvedValue(Response.json({ ...initial, revision: 2 }));
    const client = new QueryClient(); const options = receiptQuery('/stores/shop/orders/O-000001', initial, read);
    const observer = new QueryObserver(client, options); const unsubscribe = observer.subscribe(() => {});
    expect(observer.getCurrentResult().data).toEqual(initial); expect(read).not.toHaveBeenCalled();
    await observer.refetch({ throwOnError: true });
    expect(observer.getCurrentResult().data?.revision).toBe(2);
    expect(read).toHaveBeenCalledWith('/stores/shop/orders/O-000001/state', expect.objectContaining({ cache: 'no-store', credentials: 'same-origin', signal: expect.any(AbortSignal) }));
    expect(options.retry).toBe(false); expect(options.staleTime).toBe(Infinity);
    expect(options.refetchOnMount).toBe(false); expect(options.refetchOnWindowFocus).toBe(false); expect(options.refetchOnReconnect).toBe(false);
    unsubscribe(); client.clear();
  });
  it('does not retry a failed read or overwrite the last known order', async () => {
    const read = vi.fn().mockResolvedValue(new Response('private upstream details', { status: 403 }));
    const client = new QueryClient(); const observer = new QueryObserver(client, receiptQuery('/receipt', initial, read));
    const unsubscribe = observer.subscribe(() => {});
    await expect(observer.refetch({ throwOnError: true })).rejects.toThrow('Receipt refresh failed');
    expect(read).toHaveBeenCalledOnce(); expect(observer.getCurrentResult().data).toEqual(initial);
    expect(observer.getCurrentResult().isError).toBe(true);
    unsubscribe(); client.clear();
  });
  it('isolates page clients and cancels I/O without caching a late response', async () => {
    let signal: AbortSignal | undefined; let finish: (response: Response) => void = () => {};
    const read = vi.fn((_url: unknown, init?: RequestInit) => { signal = init?.signal as AbortSignal; return new Promise<Response>(resolve => { finish = resolve; }); });
    const client = new QueryClient(); const another = new QueryClient();
    const options = receiptQuery('/receipt', initial, read); const observer = new QueryObserver(client, options);
    const unsubscribe = observer.subscribe(() => {}); const refresh = observer.refetch();
    expect(signal?.aborted).toBe(false); expect(another.getQueryData(options.queryKey)).toBeUndefined();
    await client.cancelQueries({ queryKey: options.queryKey, exact: true });
    expect(signal?.aborted).toBe(true);
    finish(Response.json({ ...initial, revision: 99 })); await refresh;
    expect(observer.getCurrentResult().data).toEqual(initial);
    unsubscribe(); client.clear(); another.clear(); expect(client.getQueryCache().getAll()).toHaveLength(0);
    expect(receiptQuery('/other-receipt', initial).queryKey).not.toEqual(options.queryKey);
  });
});
