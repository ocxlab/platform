export type LiveStatus = 'connecting' | 'connected' | 'reconnecting' | 'paused';
type Source = { addEventListener(type: string, listener: () => void): void; close(): void };

// One source per visible page, one refresh in flight, and a trailing refresh when
// changes arrive during a read. There is no timer that polls order state.
export function liveUpdates(url: string, refresh: () => Promise<unknown>, status: (value: LiveStatus) => void,
  create: (url: string) => Source = url => new EventSource(url)) {
  let source: Source | undefined;
  let refreshing = false;
  let pending = false;
  let stopped = false;
  let retry: ReturnType<typeof setTimeout> | undefined;
  async function changed() {
    pending = true;
    if (refreshing || stopped) return;
    refreshing = true;
    try {
      while (pending && !stopped) { pending = false; await refresh(); }
    } catch {
      if (!stopped && source) {
        status('reconnecting'); source.close(); source = undefined; pending = false;
        // Retry the broken live connection, not periodic reads. Its initial hint
        // will request fresh state once the transport is re-established.
        retry = setTimeout(() => { retry = undefined; connect(); }, 3000);
      }
    }
    finally { refreshing = false; }
  }
  function connect() {
    if (source || stopped || retry) return;
    status('connecting');
    const current = source = create(url);
    current.addEventListener('open', () => { if (source === current) status('connected'); });
    current.addEventListener('error', () => { if (source === current) status('reconnecting'); });
    current.addEventListener('invalidate', () => { if (source === current) void changed(); });
  }
  return {
    visible(value: boolean) {
      if (value) connect();
      else { clearTimeout(retry); retry = undefined; source?.close(); source = undefined; status('paused'); }
    },
    close() { stopped = true; pending = false; clearTimeout(retry); source?.close(); source = undefined; },
  };
}
