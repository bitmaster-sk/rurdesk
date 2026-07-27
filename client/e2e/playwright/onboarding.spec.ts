import { test, expect } from '@playwright/test';
import { BOOTSTRAP_USER, ensureBootstrapUser } from './support/bootstrap-user';

// The first-project onboarding only renders for an AUTHENTICATED user who owns
// zero projects. We establish that precondition ourselves: ensure the instance
// bootstrap user exists (first user → instance admin, owns no projects), then log
// in through the real UI so the app stores the token the way it expects. The
// login flow navigates to '/', where the firstProjectGuard matches onboarding.
test('a user with no projects sees the first-project onboarding at /', async ({
    page,
    request,
    baseURL
}) => {
    await ensureBootstrapUser(request, baseURL!);

    await page.goto('/login');
    await page.locator('input[formcontrolname="email"]').fill(BOOTSTRAP_USER.email);
    await page.locator('input[formcontrolname="password"]').fill(BOOTSTRAP_USER.password);
    await page.getByRole('button', { name: /login/i }).click();

    await expect(page.getByText(/create your first project/i)).toBeVisible();
    await expect(page.getByRole('button', { name: /create & generate backlog/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /create blank project/i })).toBeVisible();
});
