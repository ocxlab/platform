import { assign, createActor, fromPromise, setup } from 'xstate';
import type { components } from '$lib/api/generated';

export type CheckedBasket = { quote: components['schemas']['FoodQuote']; intent: { quote: string; key: string } };
type Context = { desired: string; requested: string; checked?: CheckedBasket; error: boolean };
type Event = { type: 'CHANGE'; input: string } | { type: 'RETRY' | 'PLACE' | 'REJECTED' | 'REDIRECTED' };

// Quote POSTs write an HttpOnly intent cookie. Never overlap them: finish the
// current request, discard stale data, then validate only the newest selection.
export function basketCheckout(check: (input: string, signal: AbortSignal) => Promise<CheckedBasket>) {
  return createActor(setup({
    types: { context: {} as Context, events: {} as Event },
    actors: { quote: fromPromise<CheckedBasket, string>(({ input, signal }) => check(input, signal)) },
    actions: {
      change: assign({ desired: ({ context, event }) => event.type === 'CHANGE' ? event.input : context.desired }),
      request: assign({ requested: ({ context }) => context.desired, error: false }),
      failed: assign({ error: true }),
      clear: assign({ checked: undefined, requested: '', error: false }),
    },
    guards: {
      empty: ({ context }) => !context.desired,
      stale: ({ context }) => context.desired !== context.requested,
    },
  }).createMachine({
    id: 'basket', initial: 'idle', context: { desired: '', requested: '', error: false },
    on: { CHANGE: { target: '.debouncing', actions: 'change' } },
    states: {
      idle: { entry: 'clear' },
      debouncing: {
        always: { guard: 'empty', target: 'idle' },
        after: { 350: 'quoting' },
      },
      quoting: {
        entry: 'request',
        on: { CHANGE: { actions: 'change' } },
        invoke: {
          src: 'quote', input: ({ context }) => context.requested,
          onDone: [
            { guard: 'stale', target: 'debouncing' },
            { target: 'ready', actions: assign({ checked: ({ event }) => event.output }) },
          ],
          onError: [{ guard: 'stale', target: 'debouncing' }, { target: 'quoteFailed', actions: 'failed' }],
        },
      },
      ready: { on: { PLACE: 'placing', RETRY: 'debouncing' } },
      quoteFailed: { on: { RETRY: 'debouncing' } },
      // Placement is submitted once by the native enhanced form, never retried
      // by an actor. An intentional retry keeps the same quote/idempotency key.
      placing: { on: { CHANGE: { actions: 'change' }, REJECTED: 'placeFailed', REDIRECTED: 'complete' } },
      placeFailed: { on: { PLACE: 'placing', RETRY: 'debouncing' }, always: { guard: 'stale', target: 'debouncing' } },
      complete: { type: 'final' },
    },
  }));
}
