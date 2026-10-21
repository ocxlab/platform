import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';
import { svelteTesting } from '@testing-library/svelte/vite';
export default defineConfig({
  plugins: [sveltekit(), svelteTesting()],
  // Compile icon components through the same Svelte pipeline as the app. Deep
  // icon imports otherwise enter Vite's incremental prebundle independently.
  optimizeDeps: { exclude: ['@lucide/svelte'] },
  test: { include: ['src/**/*.test.ts'], environment: 'jsdom', setupFiles: ['./src/test-setup.ts'] },
});
