import { afterEach, expect, it, vi } from 'vitest';
vi.mock('$env/dynamic/private', () => ({ env: { OCP_API_URL: 'http://api.test' } }));
import { orderUpdates } from './order-updates';

afterEach(() => vi.unstubAllGlobals());
it('keeps API credentials server-side and forwards only the event stream', async () => {
  const fetcher = vi.fn().mockResolvedValue(new Response('event: invalidate\ndata: {}\n\n', { headers: { 'Content-Type': 'text/event-stream', 'Authorization': 'must-not-forward' } }));
  vi.stubGlobal('fetch', fetcher);
  const request = new Request('http://web.test/stores/store-1/orders/O-000001/updates');
  const response = await orderUpdates(request, '/api/v1/guest/scoped-updates', 'test-bearer');
  expect(fetcher).toHaveBeenCalledWith('http://api.test/api/v1/guest/scoped-updates', expect.objectContaining({ headers: { Authorization: 'Bearer test-bearer', Accept: 'text/event-stream' }, redirect: 'error' }));
  expect(response.headers.get('cache-control')).toBe('no-store');
  expect(response.headers.get('authorization')).toBeNull();
  expect(await response.text()).toBe('event: invalidate\ndata: {}\n\n');
});
it('preserves authorization failure and rejects a non-stream success response', async () => {
  const fetcher = vi.fn(); vi.stubGlobal('fetch', fetcher);
  const request = new Request('http://web.test/updates');
  fetcher.mockResolvedValueOnce(new Response('{}', { status: 403 }));
  await expect(orderUpdates(request, '/fixed-path', 'token')).rejects.toMatchObject({ status: 403 });
  fetcher.mockResolvedValueOnce(new Response('{}', { headers: { 'Content-Type': 'application/json' } }));
  await expect(orderUpdates(request, '/fixed-path', 'token')).rejects.toMatchObject({ status: 502 });
});
it('propagates browser disconnect to the upstream request signal', async () => {
  const fetcher = vi.fn().mockResolvedValue(new Response('', { headers: { 'Content-Type': 'text/event-stream' } }));
  vi.stubGlobal('fetch', fetcher);
  const controller = new AbortController();
  await orderUpdates(new Request('http://web.test/updates', { signal: controller.signal }), '/fixed-path', 'token');
  const signal = fetcher.mock.calls[0][1].signal as AbortSignal;
  expect(signal.aborted).toBe(false);
  controller.abort(); expect(signal.aborted).toBe(true);
});
