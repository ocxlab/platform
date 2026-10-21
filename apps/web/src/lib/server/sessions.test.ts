import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Cookies } from '@sveltejs/kit';
import { connect, disconnect, session } from './sessions';

vi.mock('$env/dynamic/private', () => ({ env: {} }));

function cookieJar() {
  const values = new Map<string, string>();
  const set = vi.fn((name: string, value: string) => { values.set(name, value); });
  const cookies = { get: (name: string) => values.get(name), set,
    delete: (name: string) => { values.delete(name); } } as unknown as Cookies;
  return { cookies, values, set };
}

afterEach(() => vi.useRealTimers());

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
