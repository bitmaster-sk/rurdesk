import { type Page, expect, test } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Release Project';
const SHIPPED_TITLE = 'Dark mode for the task board';
const OPEN_TITLE = 'Export the board as PDF';
const DESCRIPTION = 'Created by the release e2e test.';

async function tagIssue(page: Page, idProject: number, idIssue: number): Promise<void> {
    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    const tagField = page.getByTestId('tag-input-field');
    await tagField.fill('next');
    await expect(page.getByTestId('tag-suggestion').first()).toContainText('next');
    const saved = Interaction.waitForIssueSaved(page, idProject, idIssue);
    await tagField.press('Enter');
    await saved;
}

// Scenario:
// - create a dedicated non-admin user, log in and create a blank project (the user owns it)
// - create the tag "next" in project settings
// - create a closed task and an open task and put "next" on both
// - open Releases from the project menu and start a new release
// - pick "next", name the version "v1.0.0" and start the draft
// - go back to the release list and continue the draft from its row
// - continue past the version step
// - assert the tasks step counts one closed task, links it, and warns once about the open one
// - keep the default (move the open task) and continue to the release notes
// - assert the step asks how to write the notes, choose to write them by hand
// - assert the notes start with the closed task and the preview shows it
// - assert the tag is still "next" while the release is a draft
// - release it and confirm, assert the page now shows the release as released
// - assert the released notes list the closed task
// - assert the task list of the release shows the closed task with its state, but not the open one
// - edit the released notes, save them and assert the new text is shown
// - assert the closed task now carries "v1.0.0" and the open one the recreated "next"
test('a tag is released from a draft with notes that stay editable', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'release');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);

    await page.goto(`/project/${idProject}/settings`);
    await page.getByTestId('tag-new').click();
    await page.getByTestId('tag-name').fill('next');
    await page.getByTestId('tag-save').click();
    await expect(page.getByTestId('tag-row')).toHaveCount(1);

    const idShipped = await Interaction.createIssue(page, idProject, {
        title: SHIPPED_TITLE,
        description: DESCRIPTION,
        state: 'Closed',
        severity: 'Medium'
    });
    const idOpen = await Interaction.createIssue(page, idProject, {
        title: OPEN_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });
    await tagIssue(page, idProject, idShipped);
    await tagIssue(page, idProject, idOpen);

    await page.getByTestId('sidebar-link-release').click();
    await expect(page.getByTestId('release-empty-list')).toBeVisible();
    await page.getByTestId('release-new').click();

    await page.locator('#release-tag').click();
    await page.getByRole('option').filter({ hasText: 'next' }).click();
    await page.getByTestId('release-name').fill('v1.0.0');
    await page.getByTestId('release-create').click();

    await expect(page.getByTestId('release-title')).toHaveText('v1.0.0');
    await page.getByTestId('sidebar-link-release').click();
    await expect(page.getByTestId('release-row')).toContainText('Draft');
    await page.getByTestId('release-continue').click();
    await expect(page.getByTestId('release-title')).toHaveText('v1.0.0');
    await page.getByTestId('release-next').click();

    await expect(page.getByTestId('release-closed-count')).toContainText('1');
    await expect(page.getByTestId('release-issue').first()).toContainText(SHIPPED_TITLE);
    await expect(page.getByTestId('release-tasks-warning')).toHaveCount(1);
    await expect(page.getByTestId('release-tasks-warning')).toContainText(OPEN_TITLE);
    await page.getByTestId('release-next').click();

    await expect(page.getByTestId('release-write-agent')).toBeDisabled();
    await page.getByTestId('release-write-myself').click();
    await expect(page.getByTestId('release-notes-preview')).toContainText(SHIPPED_TITLE);
    await expect(page.getByText('Draft saved')).toBeVisible();
    await expect(page.getByTestId('release-status')).toContainText('Draft');
    await page.goto(`/project/${idProject}/issue/${idShipped}`);
    await expect(page.getByTestId('tag-input')).toContainText('next');
    await page.goBack();

    await expect(page.getByTestId('release-notes-preview')).toContainText(SHIPPED_TITLE);
    await page.getByTestId('release-publish').click();
    await page.locator('.ui-confirm-panel').getByRole('button', { name: 'Release v1.0.0' }).click();

    await expect(page.getByTestId('release-status')).toContainText('Released');
    await expect(page.getByTestId('release-issue-count')).toHaveText('1');
    await expect(page.getByTestId('release-published-notes')).toContainText(SHIPPED_TITLE);
    await expect(page.getByTestId('release-task-row')).toHaveCount(1);
    await expect(page.getByTestId('release-task-row')).toContainText(SHIPPED_TITLE);
    await expect(page.getByTestId('release-task-row')).toContainText('Closed');
    await page.getByTestId('release-notes-edit-start').click();
    await Interaction.fillWikiEditor(page, '## Fixed\n\n- Polished the release (#1)\n');
    await expect(page.getByTestId('release-published-preview')).toContainText('Polished');
    await page.getByTestId('release-notes-save').click();
    await expect(page.getByTestId('release-published-notes')).toContainText('Polished');

    await page.goto(`/project/${idProject}/issue/${idShipped}`);
    await expect(page.getByTestId('tag-input')).toContainText('v1.0.0');
    await page.goto(`/project/${idProject}/issue/${idOpen}`);
    await expect(page.getByTestId('tag-input')).toContainText('next');
    await expect(page.getByTestId('tag-input')).not.toContainText('v1.0.0');
});
