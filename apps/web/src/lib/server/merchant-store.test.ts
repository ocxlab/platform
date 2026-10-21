import { beforeEach, expect, it, vi } from 'vitest';
import type { Cookies } from '@sveltejs/kit';

const mocks = vi.hoisted(() => ({ session: vi.fn(), get: vi.fn() }));
vi.mock('./sessions', () => ({ session: mocks.session }));
vi.mock('./api', () => ({ api: () => ({ GET: mocks.get }), message: () => 'Request failed', text: () => 'Not found' }));
import { merchantStore } from './merchant-store';

const cookies = {} as Cookies;
beforeEach(() => {
  vi.clearAllMocks();
  mocks.session.mockReturnValue({ merchant: 'merchant-a', token: 'scoped-token' });
});

it('resolves the readable handle within the authenticated merchant only', async () => {
  mocks.get.mockResolvedValue({ data: { items: [{ id: 'store-uuid', display_name: 'Kitchen', handle: 'store-42' }] } });
  const result = await merchantStore(cookies, 'kitchen-store-42');
  expect(mocks.get).toHaveBeenCalledWith('/api/v1/merchants/{merchant_id}/stores', {
    params: { path: { merchant_id: 'merchant-a' }, query: { handle: 'store-42', limit: 1 } },
  });
  expect(result.path).toEqual({ merchant_id: 'merchant-a', store_id: 'store-uuid' });
});

it('redirects an unauthenticated visitor before resolving any store', async () => {
  mocks.session.mockReturnValue(undefined);
  await expect(merchantStore(cookies, 'kitchen-store-42')).rejects.toMatchObject({ status: 303, location: '/merchant' });
  expect(mocks.get).not.toHaveBeenCalled();
});

it('does not interpret malformed slugs as API identifiers', async () => {
  await expect(merchantStore(cookies, 'store-42')).rejects.toMatchObject({ status: 404 });
  expect(mocks.get).not.toHaveBeenCalled();
});

it('returns not found when the authorized API omits an unknown or foreign handle', async () => {
  mocks.get.mockResolvedValue({ data: { items: [] } });
  await expect(merchantStore(cookies, 'foreign-store-42')).rejects.toMatchObject({ status: 404 });
});

it('preserves API failures instead of reporting an empty store', async () => {
  mocks.get.mockResolvedValue({ response: { status: 403 }, error: {} });
  await expect(merchantStore(cookies, 'kitchen-store-42')).rejects.toMatchObject({ status: 403 });
});
