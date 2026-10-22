import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';

const fixture = JSON.parse(process.env.OCP_BROWSER_FIXTURE ?? '{}') as {
  extras_merchant_id: string; extras_merchant_token: string; provider_token: string;
};

test('admin browses paged records and customers select paid and free pizza extras', async ({ page, browser, request }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', entry => { if (entry.text().includes('Failed to hydrate')) errors.push(entry.text()); });
  // A separate synthetic merchant leaves the existing publication regression unchanged.
  let store = '';
  for (let i = 0; i < 12; i++) {
    const result = await request.post(`http://browser-api:8080/api/v1/merchants/${fixture.extras_merchant_id}/stores`, {
      headers: { Authorization: `Bearer ${fixture.extras_merchant_token}`, 'Idempotency-Key': randomUUID() },
      data: { display_name: `Pizza extras test ${String(i).padStart(2,'0')}` },
    });
    expect(result.status()).toBe(201);
    if (i === 0) store = (await result.json()).id;
  }
  await page.goto('/merchant');
  await page.getByLabel('Merchant identifier', { exact: true }).fill(fixture.extras_merchant_id);
  await page.getByLabel('Temporary merchant credential').fill(fixture.extras_merchant_token);
  await page.getByRole('button', { name: 'Connect merchant' }).click();
  await page.getByRole('link', { name: 'Pizza extras test 00', exact: true }).click();
  await expect(page).toHaveURL(/\/merchant\/stores\/pizza-extras-test-00-store-[1-9][0-9]*$/);
  const workspace = page.url();
  expect(new URL(workspace).pathname).toMatch(/^\/merchant\/stores\/pizza-extras-test-00-store-[1-9][0-9]*$/);
  await page.screenshot({ path: 'test-results/merchant-tabs-desktop.png', fullPage: true });
  for (const locale of ['ar', 'en']) {
    await page.locator('.language-picker summary').click();
    await page.locator(`.language-picker button[value=${locale}]`).click();
    await expect(page.locator('html')).toHaveAttribute('lang', locale);
    await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
    await page.setViewportSize({ width: 390, height: 844 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await expect(page.locator('nav a[aria-current="page"]')).toHaveCount(1);
  }
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.getByRole('button', { name: 'Add menu item', exact: true }).click();
  await page.getByLabel('Item name', { exact: true }).fill('Pizza');
  await page.getByLabel('Variant name', { exact: true }).fill('Small');
  await page.getByLabel('Variant price in EUR', { exact: true }).fill('8');
  await page.getByRole('button', { name: 'Add variant', exact: true }).click();
  await page.getByLabel('Variant name', { exact: true }).last().fill('Large');
  await page.getByLabel('Variant price in EUR', { exact: true }).last().fill('10');
  await expect(page.getByRole('heading', { name: 'Extras and additions' })).toBeVisible();
  await page.getByRole('button', { name: 'Add extras group', exact: true }).click();
  await page.getByLabel('Extra name', { exact: true }).fill('Extra cheese');
  await page.getByLabel('Extra price in EUR', { exact: true }).fill('1');
  await page.getByRole('button', { name: 'Add extra', exact: true }).click();
  await page.getByLabel('Extra name', { exact: true }).last().fill('Garlic');
  await page.screenshot({ path: 'test-results/menu-extras-desktop.png', fullPage: true });
  await page.getByRole('button', { name: 'Save private draft' }).click();
  await expect(page.getByRole('status')).toContainText('Draft saved');
  await page.getByRole('button', { name: 'Publish saved draft' }).click();
  await expect(page.getByRole('status')).toHaveText('Menu published.');

  const guestContext = await browser.newContext();
  const guest = await guestContext.newPage();
  guest.on('pageerror', error => errors.push(error.message));
  const storefront = await page.getByRole('link', { name: 'Open customer storefront', exact: true }).getAttribute('href');
  await guest.goto(storefront!);
  await guest.getByRole('link', { name: /^Choose / }).first().click();
  await guest.getByRole('combobox', { name: /^Variant/ }).selectOption({ label: 'Large · €10.00' });
  await guest.getByLabel('Extra cheese').check();
  await guest.getByLabel('Garlic').check();
  await expect(guest.locator('label').filter({ has: guest.getByRole('checkbox', { name: /Garlic/ }) })).toContainText('€0.00');
  await guest.getByRole('button', { name: 'Check selection price' }).click();
  await expect(guest.getByRole('region', { name: 'Selection quote' })).toContainText('11.00');
  await guest.getByRole('button', { name: 'Place pickup order', exact: true }).click();
  await expect(guest.getByRole('region', { name: 'Order details' })).toContainText('Extra cheese');
  await expect(guest.getByRole('region', { name: 'Order details' })).toContainText('Garlic');
  await expect(guest.getByRole('region', { name: 'Order details' })).toContainText('11.00');

  await page.goto('/admin');
  await page.getByLabel('Temporary provider credential').fill(fixture.provider_token);
  await page.getByRole('button', { name: 'Connect provider' }).click();
  await expect(page.getByRole('textbox', { name: 'Merchant identifier', exact: true })).toHaveCount(0);
  await expect(page.locator('tbody tr')).toHaveCount(12);
  await page.getByRole('searchbox', { name: 'Search', exact: true }).fill('Pizza extras test');
  await page.getByLabel('Rows per page').selectOption('10');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(10);
  const fieldBottoms = await page.locator('.admin-filters label input, .admin-filters label select').evaluateAll(nodes => nodes.map(node => node.getBoundingClientRect().bottom));
  expect(Math.max(...fieldBottoms) - Math.min(...fieldBottoms)).toBeLessThanOrEqual(1);
  const searchBox = await page.getByRole('button', { name: 'Search', exact: true }).boundingBox();
  const clearBox = await page.getByRole('link', { name: 'Clear filters', exact: true }).boundingBox();
  expect(Math.abs(searchBox!.y - clearBox!.y)).toBeLessThanOrEqual(1);
  expect(Math.abs(searchBox!.height - clearBox!.height)).toBeLessThanOrEqual(1);
  await page.screenshot({ path: 'test-results/admin-stores-desktop.png', fullPage: true });
  await page.getByRole('link', { name: 'Next page', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(2);
  await expect(page.getByRole('searchbox', { name: 'Search', exact: true })).toHaveValue('Pizza extras test');
  await page.getByRole('link', { name: 'Previous page', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(10);
  await page.getByRole('combobox', { name: /^Status/ }).selectOption('published');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(1);
  await expect(page.getByRole('link', { name: 'Next page', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Inspect publication', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Publication visibility' })).toBeVisible();
  await expect(page.getByRole('searchbox', { name: 'Search', exact: true })).toHaveValue('Pizza extras test');
  await page.getByRole('link', { name: 'Orders', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(1);
  await page.getByRole('searchbox', { name: 'Search', exact: true }).fill('Pizza extras test 00');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(1);
  await page.getByRole('button', { name: 'Inspect order', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Order details' })).toContainText('11.00');
  await expect(page.getByRole('button', { name: 'Accept order', exact: true })).toHaveCount(0);
  await page.getByRole('link', { name: 'Refresh status', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(1);
  for (const locale of ['ar', 'en']) {
    await page.locator('.language-picker summary').click();
    await page.locator(`.language-picker button[value=${locale}]`).click();
    await expect(page.locator('html')).toHaveAttribute('dir', locale === 'ar' ? 'rtl' : 'ltr');
    for (const cell of await page.locator('th,td').all()) await expect(cell).toHaveCSS('text-align','start');
  }
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/admin-orders-mobile.png', fullPage: true });
  // Search and pagination are ordinary links/forms and remain available without JS.
  const noScript = await browser.newContext({ javaScriptEnabled: false, storageState: await page.context().storageState() });
  const plain = await noScript.newPage();
  await plain.goto('/admin?tab=stores&q=Pizza+extras+test&limit=10');
  await expect(plain.locator('tbody tr')).toHaveCount(10);
  await plain.getByRole('link', { name: 'Next page', exact: true }).click();
  await expect(plain.locator('tbody tr')).toHaveCount(2);
  // Leave no additional public store for the existing publication journey.
  await page.goto(workspace);
  await page.getByRole('button', { name: 'Withdraw menu', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('withdrawn');
  expect(errors).toEqual([]);
  await noScript.close(); await guestContext.close();
});
