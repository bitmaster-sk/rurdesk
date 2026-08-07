import { test, expect } from '@playwright/test';

// Frontend-only auth behaviour — no backend needed (AuthGuard reads the local token).
test.describe('auth guard', () => {
    // Scenario:
    // - open a protected route in a context that has no stored token
    // - assert the guard bounced the navigation to /login
    // - assert the login form rendered there
    test('redirects an unauthenticated visit to a protected route to /login', async ({ page }) => {
        // Fresh context has no stored token → guard should bounce us to login.
        await page.goto('/project');
        await expect(page).toHaveURL(/\/login$/);
        await expect(page.locator('form')).toBeVisible();
    });

    // Scenario:
    // - open /login with no token
    // - assert the guard did NOT redirect and the email field is reachable
    test('keeps the login route accessible without a token', async ({ page }) => {
        await page.goto('/login');
        await expect(page).toHaveURL(/\/login$/);
        await expect(page.locator('input[formcontrolname="email"]')).toBeVisible();
    });
});
