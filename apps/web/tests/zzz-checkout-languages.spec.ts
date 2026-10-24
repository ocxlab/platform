import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';

const fixture = JSON.parse(process.env.OCP_BROWSER_FIXTURE ?? '{}') as {
  demo_merchant_id: string; demo_merchant_token: string; demo_menu: string;
};

for (const [locale, add, place, title] of [
  ['de', 'In den Warenkorb', 'Zur Abholung bestellen', 'Abholbestellung aufgegeben'],
  ['en', 'Add to basket', 'Place pickup order', 'Pickup order placed'],
  ['ar', 'أضف إلى السلة', 'إرسال طلب للاستلام', 'أرسل طلب الاستلام'],
]) {
  test(`basket placement reaches the live receipt in ${locale}`, async ({ page, request }) => {
    const errors: string[] = [];
    page.on('pageerror', error => { errors.push(error.message); console.error(error.stack); });
    const base = `http://browser-api:8080/api/v1/merchants/${fixture.demo_merchant_id}/stores`;
    const headers = () => ({ Authorization: `Bearer ${fixture.demo_merchant_token}`, 'Idempotency-Key': randomUUID() });
    const created = await request.post(base, { headers: headers(), data: { display_name: `Checkout ${locale}` } });
    expect(created.status()).toBe(201);
    const store = await created.json();
    expect((await request.put(`${base}/${store.id}/offering`, { headers: headers(), data: {
      base_revision: 0, model: { id: 'ocp.food.menu', version: '1.0.0' }, document: JSON.parse(fixture.demo_menu),
    } })).status()).toBe(201);
    expect((await request.put(`${base}/${store.id}/publication`, { headers: headers(), data: { expected_generation: 0, revision: 1 } })).status()).toBe(201);
    await page.goto(`/stores/checkout-${locale}-${store.handle}`);
    await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
    await page.locator('.language-picker summary').click();
    await page.locator(`.language-picker button[value=${locale}]`).click();
    await expect(page.locator('html')).toHaveAttribute('lang', locale);
    await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
    await expect(page.locator('.floating-basket')).toBeVisible();
    // A receipt transition must remain a client navigation. Late Vite dependency
    // discovery used to force a document reload while lazy routes were loading.
    await page.evaluate(() => { (window as Window & { checkoutDocument?: string }).checkoutDocument = 'same-document'; });
    await page.locator('.menu-card').filter({ hasText: /Pizza Margherita/ }).first().click();
    await expect(page.locator('.dish-dialog')).toBeVisible();
    await page.getByRole('button', { name: add, exact: true }).click();
    await page.locator('.floating-basket').click();
    await expect(page.getByRole('button', { name: place, exact: true })).toBeEnabled();
    if (locale === 'en') {
      const key = await page.locator('.basket-dialog input[name=key]').inputValue();
      let attempts = 0;
      await page.route('**/stores/**?*/place', async route => {
        attempts++;
        if (attempts === 1) await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({
          type: 'failure', status: 503, data: '[{"error":1},"Temporary checkout failure"]',
        }) });
        else await route.continue();
      });
      await page.getByRole('button', { name: place, exact: true }).click();
      await expect(page.getByRole('alert')).toContainText('Temporary checkout failure');
      await expect(page.locator('.basket-dialog input[name=key]')).toHaveValue(key);
      await expect(page.locator('.basket-lines > li')).toHaveCount(1);
      expect(attempts).toBe(1); // no automatic retry of a purchase
      await page.locator('.basket-dialog form[action="?/place"]').evaluate(form => {
        (form as HTMLFormElement).requestSubmit(); (form as HTMLFormElement).requestSubmit();
      });
      await expect(page).toHaveURL(/\/orders\/O-[0-9]+$/);
      expect(attempts).toBe(2); // one deliberate retry, duplicate submit blocked
    } else {
    await page.getByRole('button', { name: place, exact: true }).click();
    }
    await expect(page).toHaveURL(/\/stores\/checkout-.*\/orders\/O-[0-9]+$/);
    await expect(page.getByRole('heading', { name: title, exact: true })).toBeVisible();
    expect(await page.evaluate(key => JSON.parse(sessionStorage.getItem(key) ?? 'null')?.lines,
      `ocp-basket:${fixture.demo_merchant_id}:${store.id}`)).toEqual([]);
    expect(await page.evaluate(() => (window as Window & { checkoutDocument?: string }).checkoutDocument)).toBe('same-document');
    await expect(page.locator('.basket-dialog')).toHaveCount(0);
    const state = await page.request.get(`${page.url()}/state`);
    expect(state.status()).toBe(200);
    expect((await state.json()).reference).toMatch(/^O-[0-9]+$/);
    expect(errors).toEqual([]);
  });
}
