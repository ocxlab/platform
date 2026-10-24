import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';
import { svelteTesting } from '@testing-library/svelte/vite';
export default defineConfig({
  // Isolate reproducible cold-start browser checks from the native dev server.
  cacheDir: process.env.OCP_BROWSER_COLD_START === 'true' ? 'node_modules/.vite-browser' : 'node_modules/.vite',
  plugins: [sveltekit(), svelteTesting()],
  // Compile icon components through the same Svelte pipeline as the app. Deep
  // icon imports otherwise enter Vite's incremental prebundle independently.
  optimizeDeps: {
    exclude: ['@lucide/svelte'],
    // These appear on lazily loaded routes. Discover them at startup rather
    // than forcing a full reload during checkout's receipt navigation.
    include: ['@fluent/bundle', 'xstate', '@tanstack/svelte-query', 'openapi-fetch'],
  },
  server: { watch: { ignored: ['**/.stryker-tmp/**', '**/reports/**', '**/test-results/**'] } },
  test: { include: ['src/**/*.test.ts'], environment: 'jsdom', setupFiles: ['./src/test-setup.ts'] },
});
