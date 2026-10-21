import { beforeEach, expect, it, vi } from 'vitest';
import type { Cookies } from '@sveltejs/kit';
const mocks = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock('./api', () => ({ api: () => ({ GET: mocks.get }), message: () => 'Request failed', text: (key: string) => key }));
vi.mock('$env/dynamic/private', () => ({ env: {} }));
import { customerStore, customerReceiptScope, rememberCustomerStore } from './customer-store';
import { loadReceipt, receiptPath } from './receipt';
const merchant = 'f095df04-80d9-4aa3-810d-979477079a28';
const store = '26b63e21-84eb-4e24-a539-8736a8e01482';
function jar() {
  const values = new Map<string, string>();
  const set = vi.fn((name: string, value: string) => values.set(name, value));
  return { values, set, cookies: { get: (name: string) => values.get(name), set } as unknown as Cookies };
}
beforeEach(() => vi.clearAllMocks());
it('resolves an exact public handle rather than scanning names or trusting the name prefix', async () => {
  mocks.get.mockResolvedValue({ data: { id: store, merchant_id: merchant, handle: 'store-1', display_name: 'New name' } });
  expect((await customerStore('old-name-store-1')).id).toBe(store);
  expect(mocks.get).toHaveBeenCalledWith('/api/v1/public/stores/{store_handle}', { params: { path: { store_handle: 'store-1' } } });
});
it('rejects malformed slugs and preserves upstream failure status', async () => {
  await expect(customerStore('bad-store-01')).rejects.toMatchObject({ status: 404 });
  expect(mocks.get).not.toHaveBeenCalled();
  mocks.get.mockResolvedValue({ response: { status: 503 }, error: {} });
  await expect(customerStore('shop-store-1')).rejects.toMatchObject({ status: 503 });
});
it('retains only a scoped routing hint so receipt access does not require publication', () => {
  const j = jar();
  rememberCustomerStore(j.cookies, { id: store, merchant_id: merchant, handle: 'store-1' });
  expect(customerReceiptScope(j.cookies, 'renamed-store-1')).toEqual({ merchant, store });
  expect(j.set).toHaveBeenCalledWith('ocp_route_store-1', `${merchant}.${store}`, expect.objectContaining({ httpOnly: true, sameSite: 'strict', secure: true, path: '/stores', maxAge: 604800 }));
  expect(() => customerReceiptScope(j.cookies, 'other-store-2')).toThrow();
  expect(mocks.get).not.toHaveBeenCalled();
});
it('rejects missing or malformed routing hints', () => {
  const j = jar();
  for (const value of ['', 'bad', `${merchant}.${store}.extra`, `bad.${store}`]) {
    j.values.set('ocp_route_store-1', value);
    expect(() => customerReceiptScope(j.cookies, 'shop-store-1')).toThrow();
  }
});
it('uses the authenticated reference lookup even when a routing cookie exists', async () => {
  const j = jar();
  rememberCustomerStore(j.cookies, { id: store, merchant_id: merchant, handle: 'store-1' });
  const route = { slug: 'shop-store-1', order: 'O-000042' };
  await expect(loadReceipt(route, j.cookies)).rejects.toMatchObject({ status: 401 });
  expect(mocks.get).not.toHaveBeenCalled();
  j.values.set(`ocp_guest_${merchant}_${store}`, 'synthetic-scoped-token');
  mocks.get.mockResolvedValue({ data: { id: 'internal-order-id' } });
  await expect(loadReceipt(route, j.cookies)).resolves.toEqual({ id: 'internal-order-id' });
  expect(mocks.get).toHaveBeenCalledWith('/api/v1/guest/merchants/{merchant_id}/stores/{store_id}/food/orders/by-reference/{order_reference}', { params: { path: { merchant_id: merchant, store_id: store, order_reference: 'O-000042' } } });
  expect(receiptPath(route)).toBe('/stores/shop-store-1/orders/O-000042');
});
it('does not convert denied reference reads into successful or public reads', async () => {
  const j = jar();
  rememberCustomerStore(j.cookies, { id: store, merchant_id: merchant, handle: 'store-1' });
  j.values.set(`ocp_guest_${merchant}_${store}`, 'synthetic-scoped-token');
  mocks.get.mockResolvedValue({ response: { status: 404 }, error: {} });
  await expect(loadReceipt({ slug: 'shop-store-1', order: 'O-000043' }, j.cookies)).rejects.toMatchObject({ status: 404 });
  await expect(loadReceipt({ slug: 'shop-store-1', order: 'not-an-order' }, j.cookies)).rejects.toMatchObject({ status: 404 });
  expect(mocks.get).toHaveBeenCalledTimes(1);
});
