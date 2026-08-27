import { test, expect } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Issue Type Project';
const BUG_TITLE = 'Login fails on the second attempt';
const FEATURE_TITLE = 'Export the timesheet to CSV';
const DESCRIPTION = 'Created by the issue type e2e test.';

// Scenario:
// - create a dedicated non-admin user through the admin API
// - log in through the real login form
// - create a blank project from the first-project onboarding
// - open project settings and assert the three seeded types are listed
// - add a fourth type "Spike" and assert it appears at the end
// - create two issues, one typed "Bug" and one typed "Feature"
// - assert the table's type column shows the type each issue was given
// - reload the detail of the Bug issue and assert the type survived the round trip
test('a task keeps the type it was given, and the table shows it', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'issue-type');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);

    await page.goto(`/project/${idProject}/settings`);
    const typeRows = page.locator('[data-testid="issue-type-row"]');
    await expect(typeRows).toHaveCount(3);
    await expect(typeRows).toHaveText([/Bug/, /Feature/, /Task/]);

    await page.getByRole('button', { name: /new type/i }).click();
    await page.locator('#issue-type-name').fill('Spike');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(typeRows).toHaveCount(4);
    await expect(typeRows.nth(3)).toHaveText(/Spike/);

    const idBug = await Interaction.createIssue(page, idProject, {
        title: BUG_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium',
        issueType: 'Bug'
    });
    await Interaction.createIssue(page, idProject, {
        title: FEATURE_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium',
        issueType: 'Feature'
    });

    await page.goto(`/project/${idProject}/issue/view/table`);
    const bugRow = page.locator('tbody tr', {
        has: page.getByRole('link', { name: BUG_TITLE, exact: true })
    });
    await expect(bugRow.locator('[data-testid="issue-type-cell"]')).toHaveText('Bug');

    const featureRow = page.locator('tbody tr', {
        has: page.getByRole('link', { name: FEATURE_TITLE, exact: true })
    });
    await expect(featureRow.locator('[data-testid="issue-type-cell"]')).toHaveText('Feature');

    await page.goto(`/project/${idProject}/issue/${idBug}`);
    await expect(page.locator('#issue-type')).toHaveText(/Bug/);
});

// Scenario:
// - create a dedicated non-admin user and log in
// - create a blank project and two issues, one typed "Bug", one left untyped
// - open the table view and reveal the filter panel
// - assert "display tasks with unset types" starts checked, matching the default filter
// - filter by type "Bug" and assert the untyped task is still listed alongside it
// - untick the toggle and assert only the typed task remains
test('the unset-types toggle starts on, so filtering by type keeps untyped tasks', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'typed-tasks');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);

    await Interaction.createIssue(page, idProject, {
        title: BUG_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium',
        issueType: 'Bug'
    });
    await Interaction.createIssue(page, idProject, {
        title: FEATURE_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    await page.goto(`/project/${idProject}/issue/view/table`);
    await expect(page.locator('tbody tr')).toHaveCount(2);

    await page.getByRole('button', { name: 'Filter', exact: true }).click();

    const unsetToggle = page.locator('#issue-type-unset-filter');
    await expect(unsetToggle).toBeChecked();

    await Interaction.pickOption(page, '#issue-type-filter', 'Bug');
    await page.keyboard.press('Escape');
    await expect(page.locator('tbody tr')).toHaveCount(2);

    await unsetToggle.uncheck();
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toContainText(BUG_TITLE);
});

// Scenario:
// - create a dedicated non-admin user and log in
// - create a blank project and one task typed "Bug"
// - open project settings and delete the "Bug" type
// - the dialog reports the task using it, so pick "leave existing items unassigned"
// - assert the type is gone from settings
// - assert the task in the table now shows no type
test('deleting a type in use can leave the tasks untyped', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'type-unassign');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    await Interaction.createIssue(page, idProject, {
        title: BUG_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium',
        issueType: 'Bug'
    });

    await page.goto(`/project/${idProject}/settings`);
    const typeRows = page.locator('[data-testid="issue-type-row"]');
    await expect(typeRows).toHaveCount(3);

    await typeRows.filter({ hasText: 'Bug' }).locator('[data-testid="issue-type-delete"]').click();

    await page.locator('[data-testid="delete-migration-mode-unassign"]').check();
    await page.locator('[data-testid="delete-migration-confirm"]').click();

    await expect(typeRows).toHaveCount(2);
    await expect(typeRows).toHaveText([/Feature/, /Task/]);

    await page.goto(`/project/${idProject}/issue/view/table`);
    const row = page.locator('tbody tr', {
        has: page.getByRole('link', { name: BUG_TITLE, exact: true })
    });
    await expect(row.locator('[data-testid="issue-type-cell"]')).toContainText('—');
});

// Scenario:
// - create a dedicated non-admin user and log in
// - create a blank project and one task typed "Bug"
// - open project settings and delete the "Bug" type
// - choose to migrate its tasks and pick "Feature" as the target
// - assert the type is gone from settings
// - assert the task in the table now shows "Feature", not an empty type
test('deleting a type in use can migrate the tasks to another type', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'type-migrate');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    await Interaction.createIssue(page, idProject, {
        title: BUG_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium',
        issueType: 'Bug'
    });

    await page.goto(`/project/${idProject}/settings`);
    const typeRows = page.locator('[data-testid="issue-type-row"]');
    await typeRows.filter({ hasText: 'Bug' }).locator('[data-testid="issue-type-delete"]').click();

    await page.locator('[data-testid="delete-migration-mode-migrate"]').check();
    await Interaction.pickOption(page, '#delete-migration-target', 'Feature');
    await page.locator('[data-testid="delete-migration-confirm"]').click();

    await expect(typeRows).toHaveCount(2);

    await page.goto(`/project/${idProject}/issue/view/table`);
    const row = page.locator('tbody tr', {
        has: page.getByRole('link', { name: BUG_TITLE, exact: true })
    });
    await expect(row.locator('[data-testid="issue-type-cell"]')).toHaveText('Feature');
});
