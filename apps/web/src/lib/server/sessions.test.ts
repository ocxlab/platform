import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Cookies } from '@sveltejs/kit';
import { connect, disconnect, session } from './sessions';
import { env } from '$env/dynamic/private';

vi.mock('$env/dynamic/private', () => ({ env: {} }));

const jars: Cookies[] = [];
function cookieJar() {
  const values = new Map<string, string>();
  const set = vi.fn((name: string, value: string) => { values.set(name, value); });
  const remove=vi.fn((name: string) => { values.delete(name); });
  const cookies = { get: (name: string) => values.get(name), set, delete: remove } as unknown as Cookies;
  jars.push(cookies);
  return { cookies, values, set, remove };
}

afterEach(() => { for(const jar of jars) { disconnect(jar,'merchant'); disconnect(jar,'provider'); } jars.length=0; vi.useRealTimers(); delete env.OCP_LOCAL_INSECURE_COOKIES; });

describe('development session boundary', () => {
  it('keeps bearer credentials server-side and uses protected opaque cookies', () => {
    const jar = cookieJar();
    connect(jar.cookies, 'merchant', 'synthetic-merchant-bearer', 'merchant-id');
    expect(jar.values.get('ocp_merchant_session')).not.toContain('synthetic');
    expect(jar.set).toHaveBeenCalledWith('ocp_merchant_session', expect.any(String),
      expect.objectContaining({ httpOnly: true, sameSite: 'strict', secure: true }));
    expect(session(jar.cookies, 'merchant')?.merchant).toBe('merchant-id');
    disconnect(jar.cookies, 'merchant');
    expect(session(jar.cookies, 'merchant')).toBeUndefined();
  });

  it('rejects a merchant handle copied into the provider cookie', () => {
    const jar = cookieJar();
    connect(jar.cookies, 'merchant', 'synthetic-merchant-bearer');
    jar.values.set('ocp_provider_session', jar.values.get('ocp_merchant_session')!);
    expect(session(jar.cookies, 'provider')).toBeUndefined();
  });

  it('expires a session after one hour', () => {
    vi.useFakeTimers();
    const jar = cookieJar();
    connect(jar.cookies, 'provider', 'synthetic-provider-bearer');
    vi.advanceTimersByTime(60 * 60 * 1000);
    expect(session(jar.cookies, 'provider')).toBeUndefined();
  });
});

it('retains sessions until the exact one-hour boundary with matching cookie lifetime',()=>{
  vi.useFakeTimers(); const jar=cookieJar();
  connect(jar.cookies,'merchant','token','merchant');
  expect(jar.set).toHaveBeenLastCalledWith('ocp_merchant_session',expect.any(String),{path:'/',httpOnly:true,sameSite:'strict',secure:true,maxAge:3600});
  vi.advanceTimersByTime(3599999); expect(session(jar.cookies,'merchant')?.token).toBe('token');
  vi.advanceTimersByTime(1); jar.remove.mockClear(); expect(session(jar.cookies,'merchant')).toBeUndefined();
  expect(jar.remove).toHaveBeenCalledWith('ocp_merchant_session',{path:'/'});
});

it('invalidates retained handles on disconnect and replacement',()=>{
  const jar=cookieJar(); connect(jar.cookies,'merchant','old');
  const old=jar.values.get('ocp_merchant_session')!;
  connect(jar.cookies,'merchant','new');
  const current=jar.values.get('ocp_merchant_session')!;
  expect(session(jar.cookies,'merchant')?.merchant).toBe('');
  jar.values.set('ocp_merchant_session',old); expect(session(jar.cookies,'merchant')).toBeUndefined();
  jar.values.set('ocp_merchant_session',current); expect(session(jar.cookies,'merchant')?.token).toBe('new');
  jar.remove.mockClear(); disconnect(jar.cookies,'merchant');
  expect(jar.remove).toHaveBeenCalledWith('ocp_merchant_session',{path:'/'});
  jar.values.set('ocp_merchant_session',current); expect(session(jar.cookies,'merchant')).toBeUndefined();
  jar.values.delete('ocp_merchant_session');
  jar.remove.mockClear(); expect(session(jar.cookies,'merchant')).toBeUndefined();
  expect(jar.remove).not.toHaveBeenCalled();
});

it('bounds session capacity and reclaims expired entries while preserving live ones',()=>{
  vi.useFakeTimers();
  const first=cookieJar(); connect(first.cookies,'merchant','old');
  vi.advanceTimersByTime(1);
  for(let i=0;i<999;i++) connect(cookieJar().cookies,'merchant',`token-${i}`);
  expect(()=>connect(cookieJar().cookies,'merchant','overflow')).toThrow('Development session capacity reached');
  vi.advanceTimersByTime(3599999);
  const last=cookieJar(); connect(last.cookies,'merchant','replacement');
  expect(session(last.cookies,'merchant')?.token).toBe('replacement');
  expect(session(first.cookies,'merchant')).toBeUndefined();
  expect(()=>connect(cookieJar().cookies,'merchant','overflow')).toThrow('Development session capacity reached');
});

it('allows local insecure cookies only through the explicit development flag',()=>{
  env.OCP_LOCAL_INSECURE_COOKIES='true'; const jar=cookieJar();
  connect(jar.cookies,'provider','token');
  expect(jar.set).toHaveBeenLastCalledWith('ocp_provider_session',expect.any(String),expect.objectContaining({secure:false}));
  env.OCP_LOCAL_INSECURE_COOKIES='false'; connect(jar.cookies,'provider','token');
  expect(jar.set).toHaveBeenLastCalledWith('ocp_provider_session',expect.any(String),expect.objectContaining({secure:true}));
});
