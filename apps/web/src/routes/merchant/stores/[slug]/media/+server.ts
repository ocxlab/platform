import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import type { components } from '$lib/api/generated';
import { merchantStore } from '$lib/server/merchant-store';
import { api, message, text } from '$lib/server/api';

// This small web boundary uses the existing HttpOnly merchant session. Only
// declarations and results pass here; the browser PUTs file bytes to S3 itself.
export const POST: RequestHandler = async ({ params, cookies, request, url }) => {
  if (request.headers.get('origin') !== url.origin) error(403);
  const current = await merchantStore(cookies, params.slug);
  let input: Record<string, unknown>;
  try { input = await request.json(); if (!input || typeof input !== 'object' || Array.isArray(input)) error(400); }
  catch { error(400); }
  if (typeof input.key !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9._:-]{15,127}$/.test(input.key)) error(400);
  const client = api(current.token);
  if (input.intent === 'start') {
    const result = await client.POST('/api/v1/merchants/{merchantId}/stores/{storeId}/media', {
      params: { path: current.path, header: { 'Idempotency-Key': input.key } },
      body: { content_type: input.content_type as components['schemas']['MediaUploadRequest']['content_type'], size_bytes: input.size_bytes as number, sha256: input.sha256 as string },
    });
    if (!result.data) return json({ error: message(result.error) }, { status: result.response.status });
    const grant = await client.POST('/api/v1/merchants/{merchantId}/stores/{storeId}/media/{mediaId}/upload-grant', { params: { path: { ...current.path, mediaId: result.data.id } } });
    if (!grant.data) return json({ error: message(grant.error) }, { status: grant.response.status });
    return json({ id: result.data.id, grant: grant.data }, { headers: { 'cache-control': 'no-store' } });
  }
  if (input.intent === 'complete' && typeof input.id === 'string') {
    const result = await client.POST('/api/v1/merchants/{merchantId}/stores/{storeId}/media/{mediaId}/completion', {
      params: { path: { ...current.path, mediaId: input.id }, header: { 'Idempotency-Key': input.key } },
    });
    if (!result.data) return json({ error: message(result.error) }, { status: result.response.status });
    return json({ id: result.data.id }, { headers: { 'cache-control': 'no-store' } });
  }
  return json({ error: text('media-failed') }, { status: 400 });
};
