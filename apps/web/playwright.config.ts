import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './tests', workers: 1, retries: 0, timeout: 60_000,
  reporter: 'list', use: { baseURL: process.env.OCP_BROWSER_URL ?? 'http://browser-web:3000',
    browserName: 'chromium', channel: process.env.OCP_BROWSER_CHANNEL,
    trace: 'off', screenshot: 'off', video: 'off' },
});
