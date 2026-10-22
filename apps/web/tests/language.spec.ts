import { test, expect } from '@playwright/test';

test('language preference persists and names keep their own direction in both layouts', async ({ page, browser, request }) => {
  page.on('pageerror', error => console.error('Browser exception:', error.stack));
  page.on('console', async entry => {
    if (entry.text().includes('Failed to hydrate')) {
      for (const argument of entry.args()) console.error(await argument.evaluate(value => value instanceof Error ? value.stack : String(value)));
    }
  });
  await page.goto('/merchant?view=test');
  await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
  await expect(page.locator('header nav a')).toHaveCount(2);
  for (const name of ['Merchant', 'Provider admin']) {
    const link = page.getByRole('link', { name, exact: true });
    await expect(link).not.toHaveAttribute('title');
    await link.hover();
    await expect(page.getByRole('tooltip')).toHaveText(name);
    await page.keyboard.press('Escape');
    await expect(page.getByRole('tooltip')).toHaveCount(0);
  }
  await expect(page.locator('.language-picker summary')).toHaveText('');
  await expect(page.locator('.language-picker summary img')).toBeVisible();
  const corner = await page.locator('.language-picker').boundingBox();
  const viewport = page.viewportSize()!;
  const content = await page.locator('main').boundingBox();
  expect(Math.abs(content!.x + content!.width - corner!.x - corner!.width)).toBeLessThan(40);
  expect(viewport.height - corner!.y - corner!.height).toBeLessThan(40);
  await page.locator('.language-picker summary').click();
  const arabic = page.locator('.language-picker button[value=ar] bdi');
  await expect(arabic).toHaveText('العربية');
  await expect(arabic).toHaveAttribute('lang', 'ar');
  await expect(arabic).toHaveCSS('direction', 'rtl');
  await expect(arabic).toHaveCSS('unicode-bidi', 'isolate');
  for (const image of await page.locator('.language-picker img').all()) expect(await image.evaluate((element) => (element as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
  await page.locator('.language-picker button[value=ar]').click();
  await expect(page).toHaveURL(/\/merchant\?view=test$/);
  await expect(page.locator('html')).toHaveAttribute('lang', 'ar');
  await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
  await expect(page.getByRole('heading', { name: 'مساحة عمل التاجر', exact: true })).toBeVisible();
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-hydrated', 'true');
  await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
  await page.locator('.language-picker summary').click();
  for (const [code, name] of [['de','Deutsch'],['en','English']]) {
    const label = page.locator(`.language-picker button[value=${code}] bdi`);
    await expect(label).toHaveText(name);
    await expect(label).toHaveCSS('direction', 'ltr');
    await expect(label).toHaveCSS('unicode-bidi', 'isolate');
    // The first Latin letter must remain to the left of the last, even in RTL.
    expect(await label.evaluate((element) => {
      const text = element.firstChild!;
      const first = document.createRange(); first.setStart(text,0); first.setEnd(text,1);
      const last = document.createRange(); last.setStart(text,text.textContent!.length-1); last.setEnd(text,text.textContent!.length);
      return first.getBoundingClientRect().left < last.getBoundingClientRect().left;
    })).toBe(true);
  }
  await page.keyboard.press('Escape');
  await expect(page.locator('.language-picker')).not.toHaveAttribute('open', '');
  await page.getByLabel('معرف التاجر', { exact: true }).fill('not-valid');
  await page.getByLabel('بيانات اعتماد التاجر المؤقتة', { exact: true }).fill('not-valid');
  await page.getByRole('button', { name: 'ربط التاجر', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('أدخل معرف التاجر');
  await expect(page.getByLabel('معرف التاجر', { exact: true })).toHaveCSS('direction','ltr');
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.locator('.language-picker summary').click();
  const bounds = await page.locator('.language-picker form').boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(0);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(390);
  await page.locator('.language-picker button[value=de]').click();
  await expect(page.getByRole('heading', { name: 'Händlerbereich', exact: true })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('dir','ltr');
  await page.getByRole('link', { name: 'Anbieterverwaltung', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Anbieterverwaltung', exact: true })).toBeVisible();
  const cookie = (await page.context().cookies()).find((value) => value.name === 'ocp_locale');
  expect(cookie?.value).toBe('de'); expect(cookie?.httpOnly).toBe(true);
  const independent = await browser.newContext({ locale:'en-GB' }); const other = await independent.newPage();
  await other.goto('/merchant');
  await expect(other.getByRole('heading', { name:'Merchant workspace' })).toBeVisible();
  await independent.close();
  const csrf = await request.post('/language', { headers: { Origin:'https://untrusted.example' }, form:{locale:'ar',return_to:'/'} });
  expect(csrf.status()).toBe(403);
});

test('language selection works without JavaScript and Arabic preferences apply on first render', async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled:false, locale:'ar-EG' });
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.locator('html')).toHaveAttribute('dir','rtl');
  await expect(page.getByRole('heading', { name:'أعمال مستقلة. مجتمعات مترابطة.' })).toBeVisible();
  await page.locator('.language-picker summary').click();
  await page.locator('.language-picker button[value=en]').click();
  await expect(page.locator('html')).toHaveAttribute('lang','en');
  await expect(page.locator('html')).toHaveAttribute('dir','ltr');
  await expect(page.getByRole('heading', { name:'Independent businesses. Connected communities.' })).toBeVisible();
  await context.close();
});
