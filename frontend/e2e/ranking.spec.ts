import { test, expect } from '@playwright/test';

test.describe('Rankings', () => {
  test('displays leaderboard on rankings page', async ({ page }) => {
    await page.goto('/rankings');
    await expect(page.getByText(/leaderboard/i)).toBeVisible();
    // Should show ranking table
    await expect(page.locator('[data-testid="ranking-row"]').first()).toBeVisible();
  });

  test('switches between ranking dimensions', async ({ page }) => {
    await page.goto('/rankings');
    // Click on PnL dimension tab
    await page.getByRole('tab', { name: /pnl/i }).click();
    // Rankings should update
    await expect(page.getByText(/pnl/i)).toBeVisible();
  });

  test('filters by user type', async ({ page }) => {
    await page.goto('/rankings');
    // Select agent filter
    await page.getByRole('button', { name: /agents/i }).click();
    // Should show only agent rankings
    await expect(page.getByText(/agent/i)).toBeVisible();
  });
});
