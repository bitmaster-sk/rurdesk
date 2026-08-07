import { test, expect } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

// The first-project onboarding only renders for an AUTHENTICATED user who owns
// zero projects. It must NOT use the bootstrap user: that one is the instance
// admin, and AclService.LoadVisibleProjects hands an admin EVERY project on the
// instance — so any project created by any other spec flips firstProjectGuard to
// false and this test fails depending on scheduling order. A freshly created
// non-admin user only ever sees its own projects, of which it has none.
// Scenario:
// - create a dedicated non-admin user through the admin API
// - log in through the real login form, which lands on /
// - assert firstProjectGuard matched and the onboarding screen rendered
// - assert both creation paths are offered: AI backlog and blank project
test('a user with no projects sees the first-project onboarding at /', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'onboarding');
    await Interaction.login(page, user);

    await expect(page.getByText(/create your first project/i)).toBeVisible();
    await expect(page.getByRole('button', { name: /create & generate backlog/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /create blank project/i })).toBeVisible();
});
