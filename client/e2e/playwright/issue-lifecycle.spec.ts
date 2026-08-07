import { test, expect } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Lifecycle Project';
const ISSUE_TITLE = 'Ship the lifecycle test';
const ISSUE_DESCRIPTION = 'Created by the issue lifecycle e2e test.';

// Scenario:
// - create a dedicated non-admin user through the admin API
// - log in through the real login form
// - create a blank project from the first-project onboarding
// - create an issue: title, description, state "New", severity "Medium"
// - assert the detail shows the state it was given
// - go back to the table and assert the issue is listed exactly once
// - open the issue from the table through its title link
// - change the state to "In progress" and wait for the autosave PATCH to return
// - reload the page and assert the state survived the round trip to the server
test('an issue created in the UI survives a reload with the state it was given', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'issue-lifecycle');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    const idIssuePublic = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: ISSUE_DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    await expect(page.locator('#issue-state .state-badge')).toHaveText('New');

    await page.goto(`/project/${idProject}/issue/view/table`);
    const row = page.locator('tbody tr', {
        has: page.getByRole('link', { name: ISSUE_TITLE, exact: true })
    });
    await expect(row).toHaveCount(1);

    await page.getByRole('link', { name: ISSUE_TITLE, exact: true }).click();
    await page.waitForURL(`**/project/${idProject}/issue/${idIssuePublic}`);

    const saved = page.waitForResponse(
        response =>
            response.url().includes(`/project/${idProject}/issue/${idIssuePublic}`) &&
            response.request().method() === 'PATCH' &&
            response.ok()
    );
    await Interaction.pickOption(page, '#issue-state', 'In progress');
    await saved;

    await page.reload();
    await expect(page.locator('#issue-state .state-badge')).toHaveText('In progress');
});
