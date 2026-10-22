import type { components } from '$lib/api/generated';

export async function uploadMenuPhoto(endpoint: string, file: File, key: string = crypto.randomUUID()): Promise<string> {
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size < 1 || file.size > 10 * 1024 * 1024) throw new Error('invalid_media');
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', await file.arrayBuffer()));
  const sha256 = btoa(String.fromCharCode(...digest));
  const start = await fetch(endpoint, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ intent: 'start', key, content_type: file.type, size_bytes: file.size, sha256 }) });
  if (!start.ok) throw new Error('media_upload_failed');
  const { id, grant } = await start.json() as { id: string; grant: components['schemas']['MediaUploadGrant'] };
  // Do not attempt to set Content-Length or Host: browsers own those headers.
  const put = await fetch(grant.url, { method: 'PUT', headers: grant.headers, body: file, credentials: 'omit', referrerPolicy: 'no-referrer' });
  if (!put.ok) throw new Error('media_upload_failed');
  const complete = await fetch(endpoint, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ intent: 'complete', id, key: key + '-complete' }) });
  if (!complete.ok) throw new Error('media_upload_failed');
  return id;
}
