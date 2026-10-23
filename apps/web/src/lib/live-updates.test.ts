import { afterEach, describe, expect, it, vi } from 'vitest';
import { liveUpdates } from './live-updates';

describe('push-driven order updates', () => {
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); });
  it('coalesces concurrent hints, pauses hidden pages, reconnects, and disposes', async () => {
    const sources: Array<{ target: EventTarget; close: ReturnType<typeof vi.fn> }> = [];
    const create = vi.fn(() => { const source = { target: new EventTarget(), close: vi.fn() }; sources.push(source); return { addEventListener: source.target.addEventListener.bind(source.target), close: source.close }; });
    let finish: () => void = () => {};
    const refresh = vi.fn(() => new Promise<void>(resolve => { finish = resolve; }));
    const status = vi.fn();
    const updates = liveUpdates('/updates', refresh, status, create);
    updates.visible(true); updates.visible(true);
    expect(create).toHaveBeenCalledTimes(1);
    sources[0].target.dispatchEvent(new Event('open'));
    expect(status).toHaveBeenLastCalledWith('connected');
    for (let i=0;i<10;i++) sources[0].target.dispatchEvent(new Event('invalidate'));
    expect(refresh).toHaveBeenCalledTimes(1);
    finish(); await Promise.resolve();
    expect(refresh).toHaveBeenCalledTimes(2);
    finish(); await Promise.resolve();
    sources[0].target.dispatchEvent(new Event('error'));
    expect(status).toHaveBeenLastCalledWith('reconnecting');
    updates.visible(false); expect(sources[0].close).toHaveBeenCalledOnce();
    sources[0].target.dispatchEvent(new Event('invalidate'));
    expect(refresh).toHaveBeenCalledTimes(2);
    updates.visible(true); expect(create).toHaveBeenCalledTimes(2);
    updates.close(); expect(sources[1].close).toHaveBeenCalledOnce();
    sources[1].target.dispatchEvent(new Event('invalidate'));
    expect(refresh).toHaveBeenCalledTimes(2);
  });
  it('reports failed reads without starting a polling loop', async () => {
    vi.useFakeTimers();
    const target = new EventTarget(); const status = vi.fn();
    const refresh = vi.fn().mockRejectedValue(new Error('offline'));
    const create = vi.fn(() => ({ addEventListener: target.addEventListener.bind(target), close: vi.fn() }));
    const updates = liveUpdates('/updates', refresh, status, create);
    updates.visible(true); target.dispatchEvent(new Event('invalidate')); await Promise.resolve();
    expect(status).toHaveBeenLastCalledWith('reconnecting'); expect(refresh).toHaveBeenCalledOnce();
    await vi.advanceTimersByTimeAsync(3000);
    expect(create).toHaveBeenCalledTimes(2);
    expect(refresh).toHaveBeenCalledOnce(); // a new hint, not a timer, triggers reads
    updates.close();
    vi.useRealTimers();
  });
  it('reads on open and native reconnect even if no notification survives', async () => {
    vi.useFakeTimers();
    const target = new EventTarget(); const close = vi.fn(); const status = vi.fn();
    const refresh = vi.fn().mockResolvedValue(undefined);
    const create = vi.fn(() => ({ addEventListener: target.addEventListener.bind(target), close }));
    const updates = liveUpdates('/updates', refresh, status, create);
    expect(status).toHaveBeenLastCalledWith('paused');
    updates.visible(true); expect(status).toHaveBeenLastCalledWith('connecting');
    target.dispatchEvent(new Event('open')); await vi.advanceTimersByTimeAsync(0);
    expect(refresh).toHaveBeenCalledOnce();
    target.dispatchEvent(new Event('error'));
    expect(status).toHaveBeenLastCalledWith('reconnecting');
    target.dispatchEvent(new Event('open')); await vi.advanceTimersByTimeAsync(0);
    expect(refresh).toHaveBeenCalledTimes(2);
    expect(status).toHaveBeenLastCalledWith('connected');
    await vi.advanceTimersByTimeAsync(60_000);
    expect(refresh).toHaveBeenCalledTimes(2); expect(create).toHaveBeenCalledOnce();
    updates.refresh(); await vi.advanceTimersByTimeAsync(0);
    expect(refresh).toHaveBeenCalledTimes(3);
    updates.close(); expect(close).toHaveBeenCalledOnce();
  });
  it('aborts a hidden-page read and ignores its late completion and old hints', async () => {
    vi.useFakeTimers();
    const targets: EventTarget[] = []; const closes: ReturnType<typeof vi.fn>[] = [];
    const signals: AbortSignal[] = []; const finishes: Array<() => void> = [];
    const refresh = vi.fn((signal: AbortSignal) => { signals.push(signal); return new Promise<void>(resolve => finishes.push(resolve)); });
    const create = () => { const target = new EventTarget(); const close = vi.fn(); targets.push(target); closes.push(close); return { addEventListener: target.addEventListener.bind(target), close }; };
    const updates = liveUpdates('/updates', refresh, vi.fn(), create);
    updates.visible(true); targets[0].dispatchEvent(new Event('open'));
    targets[0].dispatchEvent(new Event('invalidate')); // queued, but pause discards it
    updates.visible(false); expect(signals[0].aborted).toBe(true); expect(closes[0]).toHaveBeenCalledOnce();
    finishes[0](); targets[0].dispatchEvent(new Event('invalidate')); await vi.advanceTimersByTimeAsync(0);
    expect(refresh).toHaveBeenCalledOnce();
    updates.visible(true); targets[1].dispatchEvent(new Event('open'));
    expect(refresh).toHaveBeenCalledTimes(2); expect(signals[1].aborted).toBe(false);
    updates.close(); expect(signals[1].aborted).toBe(true); expect(closes[1]).toHaveBeenCalledOnce();
    finishes[1](); targets[1].dispatchEvent(new Event('open')); await vi.advanceTimersByTimeAsync(0);
    expect(refresh).toHaveBeenCalledTimes(2);
  });
  it('cancels connection retry when hidden or disposed and handles construction failure', async () => {
    vi.useFakeTimers();
    const create = vi.fn(() => { throw new Error('unavailable'); }); const refresh = vi.fn(); const status = vi.fn();
    const updates = liveUpdates('/updates', refresh, status, create);
    updates.visible(true); expect(status).toHaveBeenLastCalledWith('reconnecting');
    updates.visible(true); expect(create).toHaveBeenCalledOnce();
    updates.visible(false); await vi.advanceTimersByTimeAsync(10_000);
    expect(create).toHaveBeenCalledOnce(); expect(status).toHaveBeenLastCalledWith('paused');
    updates.visible(true); expect(create).toHaveBeenCalledTimes(2);
    updates.close(); await vi.advanceTimersByTimeAsync(10_000);
    expect(create).toHaveBeenCalledTimes(2); expect(refresh).not.toHaveBeenCalled();
  });
  it('uses the browser EventSource and queues a fresh read when reconnect happens during a read', async () => {
    vi.useFakeTimers();
    const target = new EventTarget(); const close = vi.fn(); const urls: string[] = [];
    vi.stubGlobal('EventSource', class {
      constructor(url: string) { urls.push(url); }
      addEventListener = target.addEventListener.bind(target);
      close = close;
    });
    let finish = () => {};
    const refresh = vi.fn(() => new Promise<void>(resolve => { finish = resolve; }));
    const updates = liveUpdates('/receipt/updates', refresh, vi.fn());
    updates.visible(true); expect(urls).toEqual(['/receipt/updates']);
    target.dispatchEvent(new Event('open'));
    target.dispatchEvent(new Event('error')); target.dispatchEvent(new Event('open'));
    expect(refresh).toHaveBeenCalledOnce();
    finish(); await vi.advanceTimersByTimeAsync(0);
    expect(refresh).toHaveBeenCalledTimes(2);
    finish(); await vi.advanceTimersByTimeAsync(0);
    expect(refresh).toHaveBeenCalledTimes(2);
    updates.close(); expect(close).toHaveBeenCalledOnce();
  });
});
