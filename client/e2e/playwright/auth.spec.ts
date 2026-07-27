import { test, expect } from '@playwright/test';

// Frontend-only auth behaviour — no backend needed (AuthGuard reads the local token).
test.describe('auth guard', () => {
    test('redirects an unauthenticated visit to a protected route to /login', async ({ page }) => {
        // Fresh context has no stored token → guard should bounce us to login.
        await page.goto('/project');
        await expect(page).toHaveURL(/\/login$/);
        await expect(page.locator('form')).toBeVisible();
    });

    test('keeps the login route accessible without a token', async ({ page }) => {
        await page.goto('/login');
        await expect(page).toHaveURL(/\/login$/);
        await expect(page.locator('input[formcontrolname="email"]')).toBeVisible();
    });
});
