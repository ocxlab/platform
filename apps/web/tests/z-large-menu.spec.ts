import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';

const fixture = JSON.parse(process.env.OCP_BROWSER_FIXTURE ?? '{}') as {
  demo_merchant_id: string; demo_merchant_token: string; demo_menu: string;
};

test('48-product fixture loads for merchants and guests and prices a configured pizza', async ({ page, request }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  const base = `http://browser-api:8080/api/v1/merchants/${fixture.demo_merchant_id}/stores`;
  const headers = () => ({ Authorization: `Bearer ${fixture.demo_merchant_token}`, 'Idempotency-Key': randomUUID() });
  const created = await request.post(base, { headers: headers(), data: { display_name: 'Large menu demo' } });
  expect(created.status()).toBe(201);
  const store = await created.json();
  const saved = await request.put(`${base}/${store.id}/offering`, { headers: headers(), data: {
    base_revision: 0, model: { id: 'ocp.food.menu', version: '1.0.0' }, document: JSON.parse(fixture.demo_menu),
  } });
  expect(saved.status()).toBe(201);
  const published = await request.put(`${base}/${store.id}/publication`, { headers: headers(), data: { expected_generation: 0, revision: 1 } });
  expect(published.status()).toBe(201);
  await page.goto('/merchant');
  await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
  await page.getByLabel('Merchant identifier', { exact: true }).fill(fixture.demo_merchant_id);
  await page.getByLabel('Temporary merchant credential').fill(fixture.demo_merchant_token);
  await page.getByRole('button', { name: 'Connect merchant' }).click();
  await expect(page.getByRole('row', { name: /Large menu demo/ })).toContainText('48');
  await page.getByRole('link', { name: 'Large menu demo', exact: true }).click();
  await expect(page.getByLabel('Item name', { exact: true })).toHaveCount(48);
  await page.screenshot({ path: 'test-results/large-menu-merchant.png' });
  const storefront = await page.getByRole('link', { name: 'Open customer storefront', exact: true }).getAttribute('href');
  await page.getByRole('link', { name: 'Orders', exact: true }).click();
  await page.getByLabel('Accept new orders', { exact: true }).uncheck();
  await page.getByRole('combobox', { name: /^Acceptance of new orders/ }).selectOption('automatic');
  await page.getByRole('button', { name: 'Save ordering settings', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'New orders are paused' })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/ordering-controls-mobile.png', fullPage: true });
  await page.goto(storefront!);
  await expect(page.getByText('This restaurant has paused new orders.', { exact: false })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Check selection price' })).toBeDisabled();
  // Existing customer screens are advisory; the backend test separately checks
  // that quotes issued before a pause cannot bypass authoritative enforcement.
  const resumed = await request.put(`${base}/${store.id}/food/ordering`, {
    headers: headers(), data: { expected_revision: 1, accepting_orders: true, acceptance_mode: 'automatic' },
  });
  expect(resumed.status()).toBe(201);
  await page.goto(storefront!);
  await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
  const items = page.getByRole('combobox', { name: 'Menu item', exact: true });
  await expect(items.locator('option')).toHaveCount(48);
  await expect(items.locator('option[value=baklava]')).toHaveJSProperty('disabled', true);
  await items.selectOption('pizza-margherita');
  await page.getByRole('combobox', { name: /^Variant/ }).selectOption('large');
  await page.getByLabel('Extra Käse', { exact: false }).check();
  await page.getByLabel('Knoblauch', { exact: false }).check();
  await page.getByRole('button', { name: 'Check selection price' }).click();
  await expect(page.getByRole('region', { name: 'Selection quote' })).toContainText('12.00');
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/large-menu-customer-mobile.png', fullPage: true });
  await page.getByRole('button', { name: 'Place pickup order', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Order details' })).toContainText('Pizza Margherita');
  await expect(page.getByRole('region', { name: 'Order details' })).toContainText('12.00');
  expect(errors).toEqual([]);
});
