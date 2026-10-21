import { randomUUID } from 'node:crypto';
import { env } from '$env/dynamic/private';
import type { Cookies } from '@sveltejs/kit';

// The scoped guest bearer lives only in an HttpOnly cookie and server requests.
// Unlike development staff sessions, checkout survives a web/API process restart.
export function guestCheckout(cookies: Cookies, merchant: string, store: string) {
  const path = `/stores/${merchant}/${store}`;
  const suffix = `${merchant}_${store}`;
  const options = { path, httpOnly: true, sameSite: 'strict' as const,
    secure: env.OCP_LOCAL_INSECURE_COOKIES !== 'true', maxAge: 7 * 24 * 60 * 60 };
  const tokenName = `ocp_guest_${suffix}`;
  const intentName = `ocp_intent_${suffix}`;
  return {
    token: () => cookies.get(tokenName),
    setToken: (token: string, expires: string) => cookies.set(tokenName, token, { ...options, expires: new Date(expires), maxAge: Math.max(0, Math.floor((Date.parse(expires) - Date.now()) / 1000)) }),
    clear: () => { cookies.delete(tokenName, { path }); cookies.delete(intentName, { path }); },
    intent: (): { quote: string; key: string } | undefined => {
      const value = cookies.get(intentName)?.split('.');
      return value?.length === 2 && value.every((part) => /^[a-f0-9-]{36}$/.test(part)) ? { quote: value[0], key: value[1] } : undefined;
    },
    setIntent: (quote: string) => { const key = randomUUID(); cookies.set(intentName, `${quote}.${key}`, options); return { quote, key }; },
  };
}
