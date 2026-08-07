import { test, expect } from '@playwright/test';

// Smoke test: the app shell boots and the login page renders.
// Real end-to-end journeys (login → assign agent → phases → PR, issue CRUD,
// kanban DnD) need the full stack and are not covered yet — run the suite with
// `npm run e2e:stack`, which owns an isolated API + Postgres + Redis.
// Scenario:
// - open /login on a freshly booted app shell
// - assert the form and both credential fields rendered
test('login page renders the login form', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('form')).toBeVisible();
    await expect(page.locator('input[formcontrolname="email"]')).toBeVisible();
    await expect(page.locator('input[formcontrolname="password"]')).toBeVisible();
});
