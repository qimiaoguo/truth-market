import { test, expect } from '@playwright/test';

test.describe('Market Browsing', () => {
  test('displays market list on homepage', async ({ page }) => {
    await page.goto('/');
    // Should show at least the market list section
    await expect(page.getByText(/markets/i)).toBeVisible();
  });

  test('navigates to market detail page', async ({ page }) => {
    await page.goto('/');
    // Click on a market card
    const firstMarket = page.locator('[data-testid="market-card"]').first();
    await firstMarket.click();
    // Should navigate to market detail
    await expect(page).toHaveURL(/\/market\/.+/);
  });

  test('shows outcomes with prices on market detail', async ({ page }) => {
    await page.goto('/market/test-market-1');
    // Should display outcome options with prices
    await expect(page.getByText(/yes/i)).toBeVisible();
    await expect(page.getByText(/no/i)).toBeVisible();
  });

  test('displays order book for selected outcome', async ({ page }) => {
    await page.goto('/market/test-market-1');
    // Order book should show bids and asks
    await expect(page.getByText(/bids/i)).toBeVisible();
    await expect(page.getByText(/asks/i)).toBeVisible();
  });
});
