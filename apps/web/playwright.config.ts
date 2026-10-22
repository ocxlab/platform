import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './tests', workers: 1, retries: 0, timeout: process.env.OCP_BROWSER_URL?.includes('browser-web-dev') ? 180_000 : 60_000,
  expect: { timeout: process.env.OCP_BROWSER_URL?.includes('browser-web-dev') ? 20_000 : 5_000 },
  reporter: 'list', use: { baseURL: process.env.OCP_BROWSER_URL ?? 'http://browser-web:3000',
    actionTimeout: process.env.OCP_BROWSER_URL?.includes('browser-web-dev') ? 20_000 : 10_000,
    browserName: 'chromium', channel: process.env.OCP_BROWSER_CHANNEL ?? 'chromium',
    // Docker service names are not localhost. Treat only these test origins as
    // secure so Web Crypto behaves as it does on localhost and production HTTPS.
    // Full headless Chromium honors this flag; chromium-headless-shell does not.
    launchOptions: { args: ['--unsafely-treat-insecure-origin-as-secure=http://browser-web:3000,http://browser-web-dev:3000'] },
    trace: 'off', screenshot: 'off', video: 'off' },
});
