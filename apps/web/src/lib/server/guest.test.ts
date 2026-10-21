import { afterEach, describe, expect, it, vi } from 'vitest';
import { env } from '$env/dynamic/private';
import type { Cookies } from '@sveltejs/kit';
import { guestCheckout } from './guest';
vi.mock('$env/dynamic/private', () => ({ env: {} }));

function jar() {
  const values = new Map<string,string>();
  const set=vi.fn((name:string,value:string,_options?:Record<string,unknown>)=>values.set(name,value));
  const remove=vi.fn((name:string)=>values.delete(name));
  const cookies={get:(name:string)=>values.get(name),set,delete:remove} as unknown as Cookies;
  return {cookies,values,set,remove};
}

afterEach(()=>{ vi.useRealTimers(); delete env.OCP_LOCAL_INSECURE_COOKIES; });
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

it('sets exact cookie lifetime, expiry, names, and deletion paths',()=>{
  vi.useFakeTimers(); vi.setSystemTime(new Date('2026-01-01T00:00:00Z'));
  const j=jar(); const g=guestCheckout(j.cookies,'merchant','store');
  const expires=new Date(Date.now()+60123).toISOString();
  g.setToken('token',expires);
  expect(j.set).toHaveBeenLastCalledWith('ocp_guest_merchant_store','token',{path:'/stores/merchant/store',httpOnly:true,sameSite:'strict',secure:true,expires:new Date(expires),maxAge:60});
  g.setToken('expired',new Date(Date.now()-1000).toISOString());
  expect(j.set.mock.calls.at(-1)?.[2]).toEqual(expect.objectContaining({maxAge:0}));
  const intent=g.setIntent('c814d1aa-30cb-462c-89b7-897079a052a2');
  expect(intent.quote).toBe('c814d1aa-30cb-462c-89b7-897079a052a2');
  expect(intent.key).toMatch(/^[a-f0-9-]{36}$/);
  expect(j.set).toHaveBeenLastCalledWith('ocp_intent_merchant_store',`${intent.quote}.${intent.key}`,{path:'/stores/merchant/store',httpOnly:true,sameSite:'strict',secure:true,maxAge:604800});
  g.clear();
  expect(j.remove).toHaveBeenCalledWith('ocp_guest_merchant_store',{path:'/stores/merchant/store'});
  expect(j.remove).toHaveBeenCalledWith('ocp_intent_merchant_store',{path:'/stores/merchant/store'});
});

it('accepts only two complete identifier-shaped intent parts',()=>{
  const j=jar(); const g=guestCheckout(j.cookies,'merchant','store');
  const id='c814d1aa-30cb-462c-89b7-897079a052a2';
  expect(g.intent()).toBeUndefined();
  for(const value of ['',id,`${id}.${id}.${id}`,`bad.${id}`,`${id}.bad`,`x${id}.${id}`,`${id}.${id}x`]){
    j.values.set('ocp_intent_merchant_store',value); expect(g.intent()).toBeUndefined();
  }
});

it('uses insecure local cookies only when explicitly enabled',()=>{
  env.OCP_LOCAL_INSECURE_COOKIES='true';
  const j=jar(); guestCheckout(j.cookies,'m','s').setIntent('q');
  expect(j.set.mock.calls.at(-1)?.[2]).toEqual(expect.objectContaining({secure:false}));
  env.OCP_LOCAL_INSECURE_COOKIES='false';
  guestCheckout(j.cookies,'m','s').setIntent('q');
  expect(j.set.mock.calls.at(-1)?.[2]).toEqual(expect.objectContaining({secure:true}));
});
