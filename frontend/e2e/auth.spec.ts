import { test, expect } from '@playwright/test';

test.describe('Authentication Flow', () => {
  test('displays connect wallet button on landing page', async ({ page }) => {
    await page.goto('/');
    const connectBtn = page.getByRole('button', { name: /connect wallet/i });
    await expect(connectBtn).toBeVisible();
  });

  test('shows user balance after wallet connection', async ({ page }) => {
    // This would require mocking wallet connection
    await page.goto('/');
    // After connecting, user should see their 1000U balance
    await page.getByRole('button', { name: /connect wallet/i }).click();
    // Mock wallet connection would happen here
    await expect(page.getByText(/1,?000/)).toBeVisible({ timeout: 5000 });
  });

  test('redirects unauthenticated users from protected pages', async ({ page }) => {
    await page.goto('/portfolio');
    // Should redirect to home or show auth prompt
    await expect(page).toHaveURL(/\/(#auth)?$/);
  });
});
