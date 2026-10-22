import { env } from '$env/dynamic/private';
import { error } from '@sveltejs/kit';

// Browser EventSource cannot attach our API bearer header. A same-origin endpoint
// authenticates its HttpOnly session and forwards only a fixed API path. No token
// is put in a URL, browser JavaScript, or SSE payload.
export async function orderUpdates(request: Request, path: string, token: string) {
  const upstream = await fetch(`${env.OCP_API_URL ?? 'http://api:8080'}${path}`, {
    headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
    signal: AbortSignal.any([request.signal, AbortSignal.timeout(130_000)]),
    redirect: 'error',
  });
  if (!upstream.ok || !upstream.headers.get('content-type')?.startsWith('text/event-stream')) {
    await upstream.body?.cancel();
    error(upstream.ok ? 502 : upstream.status);
  }
  return new Response(upstream.body, { headers: {
    'Content-Type': 'text/event-stream', 'Cache-Control': 'no-store', 'X-Accel-Buffering': 'no',
  } });
}
