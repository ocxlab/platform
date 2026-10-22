import { expect, type Page } from '@playwright/test';

export async function reviewBasket(page: Page) {
  if (!await page.getByRole('dialog', { name: /Your basket/ }).isVisible()) {
    await page.getByRole('button', { name: /Your basket/ }).click();
  }
  await expect(page.getByRole('button', { name: 'Place pickup order', exact: true })).toBeEnabled();
}
