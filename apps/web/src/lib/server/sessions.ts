import { randomBytes } from 'node:crypto';
import { env } from '$env/dynamic/private';
import type { Cookies } from '@sveltejs/kit';

type Kind = 'merchant' | 'provider';
type Session = { kind: Kind; token: string; merchant: string; expires: number };
const sessions = new Map<string, Session>();
const lifetime = 60 * 60 * 1000;
const cookie = (kind: Kind) => `ocp_${kind}_session`;

// Development bootstrap only. Browser receives an opaque, audience-specific handle,
// never a reusable API bearer credential. Restarting the web process signs users out.
export function connect(cookies: Cookies, kind: Kind, token: string, merchant = '') {
  for (const [id, value] of sessions) if (value.expires <= Date.now()) sessions.delete(id);
  if (sessions.size >= 1000) throw new Error('Development session capacity reached');
  disconnect(cookies, kind);
  const id = randomBytes(32).toString('base64url');
  sessions.set(id, { kind, token, merchant, expires: Date.now() + lifetime });
  cookies.set(cookie(kind), id, { path: '/', httpOnly: true, sameSite: 'strict',
    secure: env.OCP_LOCAL_INSECURE_COOKIES !== 'true', maxAge: lifetime / 1000 });
}
export function session(cookies: Cookies, kind: Kind): Session | undefined {
  const id = cookies.get(cookie(kind));
  const value = id ? sessions.get(id) : undefined;
  if (value && value.kind === kind && value.expires > Date.now()) return value;
  if (id) { sessions.delete(id); cookies.delete(cookie(kind), { path: '/' }); }
  return undefined;
}
export function disconnect(cookies: Cookies, kind: Kind) {
  const id = cookies.get(cookie(kind)); if (id) sessions.delete(id);
  cookies.delete(cookie(kind), { path: '/' });
}
