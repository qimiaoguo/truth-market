import { test, expect } from '@playwright/test';

test.describe('Trading Flow', () => {
  // All trading tests require authentication
  test.beforeEach(async ({ page }) => {
    // TODO: Set up authenticated session via API or cookie
    await page.goto('/');
  });

  test('mints outcome tokens for a market', async ({ page }) => {
    await page.goto('/market/test-market-1');
    // Click mint button
    await page.getByRole('button', { name: /mint/i }).click();
    // Enter quantity
    await page.getByLabel(/quantity/i).fill('100');
    // Confirm mint
    await page.getByRole('button', { name: /confirm/i }).click();
    // Should show success
    await expect(page.getByText(/minted/i)).toBeVisible({ timeout: 5000 });
  });

  test('places a buy limit order', async ({ page }) => {
    await page.goto('/market/test-market-1');
    // Select buy side
    await page.getByRole('tab', { name: /buy/i }).click();
    // Enter price and quantity
    await page.getByLabel(/price/i).fill('0.65');
    await page.getByLabel(/quantity/i).fill('50');
    // Submit order
    await page.getByRole('button', { name: /place order/i }).click();
    // Should show order confirmation
    await expect(page.getByText(/order placed/i)).toBeVisible({ timeout: 5000 });
  });

  test('cancels an open order', async ({ page }) => {
    await page.goto('/portfolio');
    // Find an open order and cancel it
    const cancelBtn = page.getByRole('button', { name: /cancel/i }).first();
    await cancelBtn.click();
    // Confirm cancellation
    await page.getByRole('button', { name: /confirm/i }).click();
    await expect(page.getByText(/cancelled/i)).toBeVisible({ timeout: 5000 });
  });

  test('shows positions after trading', async ({ page }) => {
    await page.goto('/portfolio');
    // Should display user positions
    await expect(page.getByText(/positions/i)).toBeVisible();
    // At least one position should be visible after minting
    await expect(page.locator('[data-testid="position-row"]').first()).toBeVisible();
  });
});
