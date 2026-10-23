import type { components } from '$lib/api/generated';
import { queryOptions } from '@tanstack/svelte-query';

export type ReceiptOrder = components['schemas']['FoodOrder'];

export function receiptQuery(receipt: string, initialOrder: ReceiptOrder, read: typeof fetch = fetch) {
  return queryOptions({
    queryKey: ['food-receipt', receipt] as const,
    initialData: initialOrder,
    // Only explicit SSE/manual hints may fetch. No retries or periodic reads.
    enabled: false,
    retry: false,
    staleTime: Infinity,
    gcTime: Infinity,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    queryFn: async ({ signal }): Promise<ReceiptOrder> => {
      const response = await read(`${receipt}/state`, { signal, cache: 'no-store', credentials: 'same-origin' });
      if (!response.ok) throw new Error('Receipt refresh failed');
      return response.json();
    },
  });
}
