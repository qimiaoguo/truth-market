import { test, expect } from '@playwright/test';

test.describe('Admin Operations', () => {
  test.beforeEach(async ({ page }) => {
    // TODO: Set up admin session
    await page.goto('/admin');
  });

  test('creates a new binary market', async ({ page }) => {
    await page.getByRole('button', { name: /create market/i }).click();
    await page.getByLabel(/title/i).fill('Will ETH hit $10k?');
    await page.getByLabel(/description/i).fill('Resolves YES if ETH exceeds $10,000');
    await page.getByLabel(/category/i).fill('crypto');
    // Select binary type
    await page.getByLabel(/binary/i).check();
    // Submit
    await page.getByRole('button', { name: /create/i }).click();
    await expect(page.getByText(/market created/i)).toBeVisible({ timeout: 5000 });
  });

  test('resolves a market with winning outcome', async ({ page }) => {
    // Navigate to a closed market
    await page.getByText(/closed markets/i).click();
    await page.locator('[data-testid="market-row"]').first().click();
    // Click resolve
    await page.getByRole('button', { name: /resolve/i }).click();
    // Select winning outcome
    await page.getByLabel(/yes/i).check();
    await page.getByRole('button', { name: /confirm/i }).click();
    await expect(page.getByText(/resolved/i)).toBeVisible({ timeout: 5000 });
  });
});
