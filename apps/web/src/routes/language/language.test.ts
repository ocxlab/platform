import { describe, expect, it, vi } from 'vitest';
import { POST } from './+server';

function event(locale: string, returnTo: string, origin = 'https://ocp.example') {
  const set = vi.fn();
  const request = new Request('https://ocp.example/language', { method: 'POST', headers: { Origin: origin }, body: new URLSearchParams({ locale, return_to:returnTo }) });
  return { value: { request, cookies: { set }, url: new URL(request.url) } as unknown as Parameters<typeof POST>[0], set };
}
describe('language preference endpoint', () => {
  it('persists a supported preference securely and returns to the same local page', async () => {
    const { value, set } = event('ar','/merchant?after=abc');
    await expect(POST(value)).rejects.toMatchObject({ status:303, location:'/merchant?after=abc' });
    expect(set).toHaveBeenCalledWith('ocp_locale','ar',expect.objectContaining({ path:'/', httpOnly:true, sameSite:'lax', secure:true, maxAge:31536000 }));
  });
  it('rejects unsupported languages and cross-origin preference writes', async () => {
    for (const [locale, origin, status] of [['fr','https://ocp.example',400],['ar','https://untrusted.example',403]] as const) {
      const { value, set } = event(locale,'/',origin);
      await expect(POST(value)).rejects.toMatchObject({ status });
      expect(set).not.toHaveBeenCalled();
    }
  });
  it('never redirects a language change to another origin', async () => {
    for (const target of ['https://untrusted.example','//untrusted.example','/\\untrusted.example','/\r\nLocation: elsewhere']) {
      await expect(POST(event('de',target).value)).rejects.toMatchObject({ status:303, location:'/' });
    }
  });
});
