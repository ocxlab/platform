import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

// Pure TypeScript/session rules do not need SvelteKit's generated route graph.
// Resolve aliases inside Stryker's sandbox, not the live dev application's root.
export default defineConfig({
  esbuild: { tsconfigRaw: { compilerOptions: { target: 'ES2022', useDefineForClassFields: true } } },
  resolve: { alias: {
    '$lib': fileURLToPath(new URL('./src/lib', import.meta.url)),
    '$env/dynamic/private': fileURLToPath(new URL('./tests/mutation-env.ts', import.meta.url)),
  } },
  test: { environment: 'node', include: ['src/lib/orders.test.ts','src/lib/money.test.ts','src/lib/server/guest.test.ts','src/lib/server/sessions.test.ts','src/lib/food-menu.test.ts','src/lib/server/food-selection.test.ts','src/lib/live-updates.test.ts','src/lib/basket-checkout.test.ts'] },
});
