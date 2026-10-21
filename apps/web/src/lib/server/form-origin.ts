// Mirror the production form boundary in development too. This concerns the
// cookie-authenticated web UI, not the bearer-authenticated commerce REST API.
export function forbiddenFormOrigin(request: Request, origin: string): boolean {
  if (!['POST', 'PUT', 'PATCH', 'DELETE'].includes(request.method)) return false;
  const type = request.headers.get('content-type')?.split(';', 1)[0].trim().toLowerCase();
  return ['application/x-www-form-urlencoded', 'multipart/form-data', 'text/plain'].includes(type ?? '')
    && request.headers.get('origin') !== origin;
}
