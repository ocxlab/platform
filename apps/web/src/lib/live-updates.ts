import { assign, createActor, fromCallback, fromPromise, setup } from 'xstate';

export type LiveStatus = 'connecting' | 'connected' | 'reconnecting' | 'paused';
type Source = { addEventListener(type: string, listener: () => void): void; close(): void };
type Event = { type: 'SHOW' | 'HIDE' | 'OPEN' | 'ERROR' | 'HINT' | 'FAILED' };

// The actor owns connection/read lifetimes, never the order record. Notifications
// are hints: serialize authorized reads and retain one trailing read per burst.
export function liveUpdates(url: string, refresh: (signal: AbortSignal) => Promise<unknown>, status: (value: LiveStatus) => void,
  create: (url: string) => Source = url => new EventSource(url)) {
  const machine = setup({
    types: { context: {} as { pending: boolean }, events: {} as Event },
    actors: {
      connection: fromCallback<Event>(({ sendBack }) => {
        let source: Source;
        try { source = create(url); } catch { sendBack({ type: 'FAILED' }); return; }
        source.addEventListener('open', () => sendBack({ type: 'OPEN' }));
        source.addEventListener('error', () => sendBack({ type: 'ERROR' }));
        source.addEventListener('invalidate', () => sendBack({ type: 'HINT' }));
        return () => source.close();
      }),
      read: fromPromise(({ signal }) => refresh(signal)),
    },
    actions: {
      queue: assign({ pending: true }),
      clear: assign({ pending: false }),
    },
    guards: { pending: ({ context }) => context.pending },
  }).createMachine({
    id: 'live', initial: 'paused', context: { pending: false },
    states: {
      paused: { on: { SHOW: 'active' } },
      active: {
        type: 'parallel',
        invoke: { src: 'connection' },
        on: { HIDE: 'paused', FAILED: 'retry' },
        states: {
          connection: {
            initial: 'connecting',
            on: { OPEN: '.connected', ERROR: '.reconnecting' },
            states: { connecting: {}, connected: {}, reconnecting: {} },
          },
          refresh: {
            initial: 'idle',
            states: {
              idle: { on: { HINT: 'reading', OPEN: 'reading' } },
              reading: {
                entry: 'clear',
                invoke: {
                  src: 'read',
                  onDone: [{ guard: 'pending', target: 'reading', reenter: true }, { target: 'idle' }],
                  onError: '#live.retry',
                },
                on: { HINT: { actions: 'queue' }, OPEN: { actions: 'queue' } },
              },
            },
          },
        },
      },
      // Retry the connection after a failed read, not periodic status polling.
      // Native EventSource owns transient transport reconnection inside active.
      retry: { after: { 3000: 'active' }, on: { HIDE: 'paused' } },
    },
  });
  const actor = createActor(machine);
  actor.subscribe(snapshot => {
    if (snapshot.matches('paused')) status('paused');
    else if (snapshot.matches('retry') || snapshot.matches({ active: { connection: 'reconnecting' } })) status('reconnecting');
    else if (snapshot.matches({ active: { connection: 'connected' } })) status('connected');
    else status('connecting');
  });
  actor.start();
  return {
    visible(value: boolean) { actor.send({ type: value ? 'SHOW' : 'HIDE' }); },
    refresh() { actor.send({ type: 'HINT' }); },
    close() { actor.stop(); },
  };
}
