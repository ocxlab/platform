import { describe, expect, it, vi } from 'vitest';
import type { Cookies } from '@sveltejs/kit';
import { guestCheckout } from './guest';
vi.mock('$env/dynamic/private', () => ({ env: {} }));

function jar() {
  const values = new Map<string,string>();
  const set=vi.fn((name:string,value:string)=>values.set(name,value));
  const cookies={get:(name:string)=>values.get(name),set,delete:(name:string)=>values.delete(name)} as unknown as Cookies;
  return {cookies,values,set};
}
describe('guest checkout cookies',()=>{
  it('retains a store-scoped bearer and original placement intent across helper instances',()=>{
    const j=jar(); const g=guestCheckout(j.cookies,'merchant','store');
    g.setToken('synthetic-private-guest',new Date(Date.now()+60000).toISOString());
    const intent=g.setIntent('c814d1aa-30cb-462c-89b7-897079a052a2');
    const resumed=guestCheckout(j.cookies,'merchant','store');
    expect(resumed.token()).toBe('synthetic-private-guest');
    expect(resumed.intent()).toEqual(intent);
    expect(j.set).toHaveBeenCalledWith(expect.any(String),expect.any(String),expect.objectContaining({path:'/stores/merchant/store',httpOnly:true,sameSite:'strict',secure:true}));
    expect(guestCheckout(j.cookies,'merchant','other').token()).toBeUndefined();
    resumed.clear(); expect(g.token()).toBeUndefined(); expect(g.intent()).toBeUndefined();
  });
});
