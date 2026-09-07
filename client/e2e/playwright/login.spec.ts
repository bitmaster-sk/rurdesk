import { test, expect } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

// Real login journey against a running backend. The spec is self-contained:
// it creates a fresh non-admin user via the admin API, then exercises the public
// login endpoint through the UI.
//
// Requires the full stack (API + Postgres + Redis); run with `npm run e2e:stack`.
test.describe('login journey', () => {
    test('logs in a real user and shows the authenticated app', async ({
        page,
        request,
        baseURL
    }) => {
        // Arrange: a user that has never logged in before.
        const user = await createUser(request, baseURL!, 'login-journey');

        // Act: fill the real login form and submit it.
        await Interaction.login(page, user);

        // Assert: we left /login and the post-login shell is visible.
        await expect(page).not.toHaveURL(/\/login/);

        // The top-menu avatar renders only after the current user resolves,
        // so its presence proves the session is live in the UI.
        await expect(page.getByTestId('avatar-trigger')).toBeVisible();
        await expect(page.getByTestId('avatar-trigger')).toHaveAttribute('aria-label', user.name);
    });

    test('shows a login error for a bad password without navigating away', async ({
        page,
        request,
        baseURL
    }) => {
        // Arrange: a real user, but we will send a wrong password.
        const user = await createUser(request, baseURL!, 'login-bad-password');

        await page.goto('/login');
        await page.locator('input[formcontrolname="email"]').fill(user.email);
        await page.locator('input[formcontrolname="password"]').fill(user.password + '-nope');
        await page.getByRole('button', { name: /login/i }).click();

        // Assert: still on /login with the visible error alert.
        await expect(page).toHaveURL(/\/login$/);
        await expect(page.locator('.login__error')).toBeVisible();
    });
});
