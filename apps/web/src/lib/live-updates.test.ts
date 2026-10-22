import { describe, expect, it, vi } from 'vitest';
import { liveUpdates } from './live-updates';

describe('push-driven order updates', () => {
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
});
