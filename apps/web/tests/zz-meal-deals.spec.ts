import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import { reviewBasket } from './support/basket';

const fixture = JSON.parse(process.env.OCP_BROWSER_FIXTURE ?? '{}') as {
  demo_merchant_id: string; demo_merchant_token: string; demo_menu: string;
};

test('meal upgrade keeps size extras and included food in basket and agreed order', async ({ page, request }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  const base = `http://browser-api:8080/api/v1/merchants/${fixture.demo_merchant_id}/stores`;
  const headers = () => ({ Authorization: `Bearer ${fixture.demo_merchant_token}`, 'Idempotency-Key': randomUUID() });
  const created = await request.post(base, { headers: headers(), data: { display_name: 'Meal deals kitchen' } });
  expect(created.status()).toBe(201);
  const store = await created.json();
  const document = JSON.parse(fixture.demo_menu);
  const pizza = document.items.find((item: { id: string }) => item.id === 'pizza-margherita');
  const large = pizza.variants.find((variant: { id: string }) => variant.id === 'large');
  const extras = pizza.option_groups.find((group: { id: string }) => group.id === 'extras');
  extras.options.find((option: { id: string }) => option.id === 'cheese').variant_prices = { large: 150 };
  document.meal_deals = [{ id: 'healthy-pizza', name: 'Healthy Pizza', surcharge_minor: 200,
    translations: { ar: 'وجبة بيتزا مع سلطة' }, main_item_ids: [pizza.id],
    addon_choices: [{ item_id: 'mixed-salad', variant_id: 'regular' }] }];
  const saved = await request.put(`${base}/${store.id}/offering`, { headers: headers(), data: {
    base_revision: 0, model: { id: 'ocp.food.menu', version: '1.0.0' }, document,
  } });
  expect(saved.status()).toBe(201);
  expect((await request.put(`${base}/${store.id}/publication`, { headers: headers(), data: { expected_generation: 0, revision: 1 } })).status()).toBe(201);
  await page.goto(`/stores/meal-deals-kitchen-${store.handle}`);
  await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
  const merchantLink = page.locator('header nav a[href="/merchant"]');
  await expect(merchantLink).not.toHaveAttribute('title');
  await merchantLink.hover();
  await expect(page.getByRole('tooltip')).toHaveClass('ocp-tooltip');
  await expect(page.getByRole('tooltip')).toHaveText('Merchant');
  await page.keyboard.press('Escape');
  await expect(page.getByRole('tooltip')).toHaveCount(0);
  await expect(page.getByRole('link', { name: /Choose.*Pizza Margherita/ })).toContainText('Meal option from');
  await page.getByRole('link', { name: /Choose.*Pizza Margherita/ }).click();
  await page.getByRole('combobox', { name: /^Variant/ }).selectOption('large');
  await page.getByLabel('Extra Käse', { exact: false }).check();
  await expect(page.getByRole('combobox', { name: 'Meal deal', exact: true })).toHaveValue('');
  await page.getByRole('spinbutton', { name: 'Quantity', exact: true }).fill('2');
  await page.getByRole('button', { name: 'Add to basket', exact: true }).click();
  await reviewBasket(page);
  await page.getByRole('button', { name: /Make it a meal from/ }).click();
  await expect(page.getByRole('spinbutton', { name: 'Quantity', exact: true })).toHaveValue('2');
  await expect(page.getByLabel('Extra Käse', { exact: false })).toBeChecked();
  await page.getByRole('combobox', { name: 'Meal deal', exact: true }).selectOption('healthy-pizza');
  await expect(page.getByRole('combobox', { name: 'Included item', exact: true })).toHaveValue('mixed-salad:regular');
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/meal-customization-mobile.png' });
  await page.getByRole('button', { name: 'Save changes', exact: true }).click();
  await reviewBasket(page);
  const basket = page.getByRole('complementary', { name: 'Your basket' });
  await expect(basket).toContainText('Healthy Pizza');
  await expect(basket).toContainText('2 ×');
  const expected = 2 * (large.price_minor + 150 + 200);
  await expect(basket.locator('.basket-total')).toContainText((expected / 100).toFixed(2));
  await page.screenshot({ path: 'test-results/meal-basket-mobile.png' });
  await page.getByRole('button', { name: 'Place pickup order', exact: true }).click();
  await expect(page).toHaveURL(/\/orders\/O-/);
  await expect(page.getByText('Healthy Pizza', { exact: false })).toBeVisible();
  const inbox = await request.get(`${base}/${store.id}/food/orders`, { headers: headers() });
  const orderID = (await inbox.json()).items[0].id;
  const order = await request.get(`${base}/${store.id}/food/orders/${orderID}`, { headers: headers() });
  expect(order.status()).toBe(200);
  const detail = await order.json();
  expect(detail.terms.total_minor).toBe(expected);
  expect(detail.terms.lines[0].meal_deal).toMatchObject({ deal_id: 'healthy-pizza', addon_item_id: 'mixed-salad', addon_quantity: 2, surcharge_minor: 200 });
  await page.locator('.language-picker summary').click();
  await page.locator('.language-picker button[value=ar]').click();
  await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/meal-order-arabic-mobile.png' });
  expect(errors).toEqual([]);
});
