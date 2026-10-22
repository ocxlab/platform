import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';

const fixture = JSON.parse(process.env.OCP_BROWSER_FIXTURE ?? '{}') as {
  demo_merchant_id: string; demo_merchant_token: string; demo_menu: string;
};

test('merchant uploads directly to private storage and publishes only a checked photo', async ({ page, request }) => {
  const base = `http://browser-api:8080/api/v1/merchants/${fixture.demo_merchant_id}/stores`;
  const headers = () => ({ Authorization: `Bearer ${fixture.demo_merchant_token}`, 'Idempotency-Key': randomUUID() });
  const created = await request.post(base, { headers: headers(), data: { display_name: 'Photo upload restaurant' } });
  expect(created.status()).toBe(201);
  const store = await created.json();
  try {
  const document = JSON.parse(fixture.demo_menu); document.items = document.items.slice(0, 1);
  const saved = await request.put(`${base}/${store.id}/offering`, { headers: headers(), data: {
    base_revision: 0, model: { id: 'ocp.food.menu', version: '1.0.0' }, document,
  } });
  expect(saved.status()).toBe(201);
  const errors: string[] = []; page.on('pageerror', error => errors.push(error.message));
  await page.goto('/merchant');
  await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
  await page.getByLabel('Merchant identifier', { exact: true }).fill(fixture.demo_merchant_id);
  await page.getByLabel('Temporary merchant credential').fill(fixture.demo_merchant_token);
  await page.getByRole('button', { name: 'Connect merchant' }).click();
  await page.getByRole('link', { name: 'Photo upload restaurant', exact: true }).click();
  await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
  expect(await page.evaluate(() => !!crypto.subtle)).toBe(true);
  const declaration = page.waitForResponse(response => response.url().endsWith('/media') && response.request().postDataJSON()?.intent === 'start');
  const put = page.waitForResponse(response => response.url().startsWith('http://object-store:9000/') && response.request().method() === 'PUT').then(response => response.status()).catch(() => 0);
  const completion = page.waitForResponse(response => response.url().endsWith('/media') && response.request().method() === 'POST' && response.request().postDataJSON()?.intent === 'complete').then(response => response.status()).catch(() => 0);
  await page.locator('input[type=file]').setInputFiles('/demo-food/doener.jpg');
  const allocation = await declaration;
  if (!allocation.ok()) throw new Error(`Media allocation status ${allocation.status()}: ${await allocation.text()}`);
  expect(await put).toBe(200);
  expect(await completion).toBe(200);
  await expect(page.getByRole('img', { name: document.items[0].name, exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Save private draft', exact: true }).click();
  const offering = await request.get(`${base}/${store.id}/offering`, { headers: headers() });
  const draft = await offering.json();
  expect(draft.document.items[0].image_id).toMatch(/^[0-9a-f-]{36}$/);
  const publicImage = `http://browser-api:8080/api/v1/public/merchants/${fixture.demo_merchant_id}/stores/${store.id}/media/${draft.document.items[0].image_id}/image`;
  expect((await request.get(publicImage, { maxRedirects: 0 })).status()).toBe(404);
  await page.getByRole('button', { name: 'Publish saved draft', exact: true }).click();
  const visible = await request.get(publicImage, { maxRedirects: 0 });
  expect(visible.status()).toBe(307);
  expect(visible.headers().location).toContain('http://object-store:9000/');
  const image = await request.get(visible.headers().location);
  expect(image.status()).toBe(200); expect(image.headers()['content-type']).toBe('image/jpeg');
  await page.getByRole('link', { name: 'Open customer storefront', exact: true }).click();
  await expect(page.getByRole('img', { name: document.items[0].name, exact: true })).toBeVisible();
  await expect.poll(() => page.getByRole('img', { name: document.items[0].name, exact: true }).evaluate((image: HTMLImageElement) => image.naturalWidth)).toBeGreaterThan(0);
  await page.screenshot({ path: 'test-results/menu-photo-card.png' });
  await page.getByRole('link', { name: /^Choose / }).click();
  await expect(page.getByRole('dialog').getByRole('heading')).toBeFocused();
  await expect.poll(() => page.getByRole('dialog').getByRole('img').evaluate((image: HTMLImageElement) => image.naturalWidth)).toBeGreaterThan(0);
  expect(errors).toEqual([]);
  } finally {
    // Other journeys intentionally begin with an empty public directory. Keep
    // this test's draft, but withdraw its publication even after a failed check.
    const current = await request.get(`${base}/${store.id}/publication`, { headers: headers() });
    if (current.ok()) {
      const publication = await current.json();
      if (publication.revision != null) {
        expect((await request.put(`${base}/${store.id}/publication`, { headers: headers(), data: { expected_generation: publication.generation } })).status()).toBe(201);
      }
    }
  }
});
