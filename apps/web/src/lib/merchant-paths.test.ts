import { describe, expect, it } from 'vitest';
import { merchantStorePath, storeHandleFromSlug } from './merchant-paths';

describe('readable merchant paths', () => {
  it('uses a readable name and stable disambiguating handle, not UUIDs', () => {
    expect(merchantStorePath({ display_name: 'Döner Karlsruhe', handle: 'store-1' })).toBe('/merchant/stores/doener-karlsruhe-store-1');
    expect(merchantStorePath({ display_name: 'Döner Karlsruhe', handle: 'store-2' })).toBe('/merchant/stores/doener-karlsruhe-store-2');
  });
  it('handles non-Latin names and punctuation without introducing path segments', () => {
    const path = merchantStorePath({ display_name: 'مطعم / كريم', handle: 'store-3' });
    expect(path.split('/')).toHaveLength(4);
    expect(storeHandleFromSlug(decodeURIComponent(path.split('/').at(-1)!))).toBe('store-3');
    expect(merchantStorePath({ display_name: '///', handle: 'store-4' })).toBe('/merchant/stores/shop-store-4');
  });
  it('rejects malformed or overflowing handles and resolves independently of the name', () => {
    expect(storeHandleFromSlug('renamed-store-10')).toBe('store-10');
    for (const slug of ['store-1', 'shop-store-0', 'shop-store-01', 'shop-store-9999999999999999999', 'shop-store-1/orders']) expect(storeHandleFromSlug(slug)).toBeUndefined();
  });
});
