import { afterEach, describe, expect, it, vi } from 'vitest';
import { webcrypto } from 'node:crypto';
import { uploadMenuPhoto } from './media-upload';

afterEach(() => vi.unstubAllGlobals());
const file = (type = 'image/jpeg', size = 3) => ({ type, size, arrayBuffer: async () => new Uint8Array([1, 2, 3]).buffer }) as File;
describe('direct menu photo upload', () => {
  it('sends declarations to the application and bytes only to the signed destination', async () => {
    vi.stubGlobal('crypto', webcrypto);
    const request = vi.fn()
      .mockResolvedValueOnce(Response.json({ id: 'image-id', grant: { url: 'https://storage.example/signed', headers: { 'Content-Type': 'image/jpeg' } } }))
      .mockResolvedValueOnce(new Response('', { status: 200 }))
      .mockResolvedValueOnce(Response.json({ id: 'image-id' }));
    vi.stubGlobal('fetch', request);
    const image = file();
    expect(await uploadMenuPhoto('/merchant/stores/demo/media', image, 'upload-request-key-01')).toBe('image-id');
    expect(request).toHaveBeenCalledTimes(3);
    const declaration = JSON.parse(request.mock.calls[0][1].body);
    expect(declaration).toMatchObject({ intent: 'start', content_type: 'image/jpeg', size_bytes: 3 });
    expect(declaration.sha256).toHaveLength(44);
    expect(request.mock.calls[1]).toEqual(['https://storage.example/signed', expect.objectContaining({ method: 'PUT', body: image, credentials: 'omit', referrerPolicy: 'no-referrer' })]);
    expect(request.mock.calls[1][1].headers).not.toHaveProperty('Content-Length');
    expect(JSON.parse(request.mock.calls[2][1].body)).toEqual({ intent: 'complete', id: 'image-id', key: 'upload-request-key-01-complete' });
  });
  it('does not declare readiness after a rejected storage upload', async () => {
    vi.stubGlobal('crypto', webcrypto);
    const request = vi.fn().mockResolvedValueOnce(Response.json({ id: 'image-id', grant: { url: 'https://storage.example/signed', headers: {} } })).mockResolvedValueOnce(new Response('', { status: 403 }));
    vi.stubGlobal('fetch', request);
    await expect(uploadMenuPhoto('/media', file(), 'upload-request-key-02')).rejects.toThrow();
    expect(request).toHaveBeenCalledTimes(2);
  });
  it('rejects empty, oversized, and non-image input before allocating storage', async () => {
    const request = vi.fn(); vi.stubGlobal('fetch', request);
    for (const image of [file('text/html'), file('image/png', 0), file('image/jpeg', 10 * 1024 * 1024 + 1)]) {
      await expect(uploadMenuPhoto('/media', image, 'upload-request-key-03')).rejects.toThrow('invalid_media');
    }
    expect(request).not.toHaveBeenCalled();
  });
});
