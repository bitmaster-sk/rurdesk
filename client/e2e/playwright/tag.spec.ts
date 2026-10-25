import { test, expect } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Tag Project';
const TAGGED_TITLE = 'Ship the release notes generator';
const PLAIN_TITLE = 'Rename the settings menu';
const DESCRIPTION = 'Created by the tag e2e test.';

// Scenario:
// - create a dedicated non-admin user, log in and create a blank project (the user owns it)
// - open project settings, create the tag "next" and assert the dialog closes
// - create two tasks
// - open the first task, type "ne" into the tags field and accept the suggestion with Tab
// - type "release-blocker" and press Enter to create a new tag straight from the detail
// - reload the detail and assert both tags survived the round trip
// - open the table and assert the tagged row shows both tags and the other row shows none
// - filter the table by "next" and assert only the tagged task remains
// - rename "next" to "v1.6.0" in settings
// - reopen the tagged task and assert it now shows "v1.6.0"
test('a tag is created, put on a task, filters the table and follows a rename', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'tags');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);

    await page.goto(`/project/${idProject}/settings`);
    await page.getByTestId('tag-new').click();
    await page.getByTestId('tag-name').fill('next');
    await page.getByTestId('tag-save').click();
    await expect(page.getByTestId('tag-name')).toHaveCount(0);
    const tagRows = page.getByTestId('tag-row');
    await expect(tagRows).toHaveCount(1);
    await expect(tagRows.first()).toContainText('next');

    const idTagged = await Interaction.createIssue(page, idProject, {
        title: TAGGED_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });
    await Interaction.createIssue(page, idProject, {
        title: PLAIN_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    await page.goto(`/project/${idProject}/issue/${idTagged}`);
    const tagField = page.getByTestId('tag-input-field');
    await tagField.fill('ne');
    await expect(page.getByTestId('tag-suggestion').first()).toContainText('next');
    const firstSave = Interaction.waitForIssueSaved(page, idProject, idTagged);
    await tagField.press('Tab');
    await firstSave;

    await tagField.fill('release-blocker');
    await expect(page.getByTestId('tag-create')).toContainText('release-blocker');
    const secondSave = Interaction.waitForIssueSaved(page, idProject, idTagged);
    await tagField.press('Enter');
    await secondSave;

    await page.reload();
    const detailTags = page.getByTestId('tag-input');
    await expect(detailTags).toContainText('next');
    await expect(detailTags).toContainText('release-blocker');

    await page.goto(`/project/${idProject}/issue/view/table`);
    const taggedRow = page.locator('tbody tr', {
        has: page.getByRole('link', { name: TAGGED_TITLE, exact: true })
    });
    const plainRow = page.locator('tbody tr', {
        has: page.getByRole('link', { name: PLAIN_TITLE, exact: true })
    });
    await expect(taggedRow.getByTestId('issue-tags')).toContainText('next');
    await expect(taggedRow.getByTestId('issue-tags')).toContainText('release-blocker');
    await expect(plainRow.getByTestId('issue-tags')).toHaveCount(0);

    await page.getByRole('button', { name: 'Filter', exact: true }).click();
    await Interaction.pickOption(page, '#tag-filter', 'next');
    await page.keyboard.press('Escape');
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toContainText(TAGGED_TITLE);

    await page.goto(`/project/${idProject}/settings`);
    await tagRows.filter({ hasText: 'next' }).getByTestId('tag-edit').click();
    await page.getByTestId('tag-name').fill('v1.6.0');
    await page.getByTestId('tag-save').click();
    await expect(tagRows.filter({ hasText: 'v1.6.0' })).toHaveCount(1);

    await page.goto(`/project/${idProject}/issue/${idTagged}`);
    await expect(page.getByTestId('tag-input')).toContainText('v1.6.0');
    await expect(page.getByTestId('tag-input')).not.toContainText('next');
});
