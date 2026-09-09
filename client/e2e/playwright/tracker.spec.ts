import { test, expect } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Tracker Project';
const ISSUE_TITLE = 'Ship the global time tracker';
const OTHER_ISSUE_TITLE = 'Write the tracker docs';
const ISSUE_DESCRIPTION = 'Created by the tracker e2e test.';
const NOTE = 'wired the popover to the API';

// Scenario:
// - create a dedicated non-admin user through the admin API and log in
// - create a blank project and an issue in it
// - assert the issue starts with no tracked time and no header control
// - start the timer from the tracker widget on the issue detail
// - assert the header control appears with the issue reference and a running clock
// - open the popover and assert it shows "#id · project", the issue title, a clock,
//   today's total, the note input and the three action buttons
// - pause from the popover and assert the control is marked paused and the clock froze
// - resume and assert the control drops the paused mark and the clock moves again
// - type a note and submit
// - assert the header control disappears without a reload
// - assert the issue detail picked up the tracked total and the timeline grew a
//   time entry carrying the note, again without a reload
// - reload and assert the tracked total and the timeline entry survived the round trip
test('a timer started on the detail page submits into the timeline and the tracked total', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'tracker-run');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    const idIssuePublic = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: ISSUE_DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    const trackedValue = page.locator('#issue-tracked');
    const control = page.getByTestId('tracker-control');
    const clock = page.getByTestId('tracker-control-clock');

    await expect(trackedValue).toHaveText('');
    await expect(control).toBeHidden();

    await page.locator('app-tracker').getByRole('button', { name: 'Start tracking' }).click();

    await expect(control).toBeVisible();
    await expect(page.getByTestId('tracker-control-issue-link')).toContainText(`#${idIssuePublic}`);
    await expect(clock).toHaveText(/00:00:\d{2}/);

    await clock.click();
    const panel = page.locator('.tracker-panel');
    await expect(panel).toBeVisible();
    await expect(panel.locator('.tp-ref')).toHaveText(`#${idIssuePublic}`);
    await expect(panel.locator('.tp-project')).toHaveText(PROJECT_NAME);
    await expect(panel.locator('.tp-title')).toHaveText(ISSUE_TITLE);
    await expect(panel.locator('.tp-elapsed')).toHaveText(/00:00:\d{2}/);
    await expect(panel.locator('.tp-summary')).toContainText('Today');
    await expect(panel.locator('.tp-summary')).toContainText('1 tasks');
    await expect(panel.getByTestId('tracker-note-input')).toHaveAttribute('placeholder', 'Note');
    await expect(panel.getByTestId('tracker-toggle-pause')).toHaveText('Pause');
    await expect(panel.getByTestId('tracker-discard')).toHaveText('Discard');
    await expect(panel.getByTestId('tracker-confirm')).toHaveText('Submit');

    await panel.getByTestId('tracker-toggle-pause').click();
    await expect(control).toHaveClass(/is-paused/);
    await expect(panel.getByTestId('tracker-toggle-pause')).toHaveText('Resume');
    const frozen = await clock.innerText();
    await page.waitForTimeout(2000);
    await expect(clock).toHaveText(frozen);

    await panel.getByTestId('tracker-toggle-pause').click();
    await expect(control).not.toHaveClass(/is-paused/);
    await expect(clock).not.toHaveText(frozen);

    await panel.getByTestId('tracker-note-input').fill(NOTE);
    await panel.getByTestId('tracker-confirm').click();

    await expect(control).toBeHidden();
    await expect(trackedValue).toHaveText(/\d+s/);

    const timeItem = page.locator('app-activity-time-item');
    await expect(timeItem).toHaveCount(1);
    await expect(timeItem.locator('.note')).toHaveText(NOTE);
    await expect(timeItem.locator('.duration-badge')).toHaveText(/\d+s/);

    const trackedAfterSubmit = await trackedValue.innerText();

    await page.reload();
    await expect(trackedValue).toHaveText(trackedAfterSubmit);
    await expect(page.locator('app-activity-time-item .note')).toHaveText(NOTE);
});

// Scenario:
// - create a dedicated non-admin user through the admin API and log in
// - create a blank project with two issues in it
// - start the timer on the first issue from its detail page
// - open the second issue and press its start button
// - assert no error is raised and a switch dialog names both tasks and the time
//   the running timer would submit
// - accept the switch
// - assert the header control now points at the second issue and its timer restarted
// - assert the first issue kept the time it had accrued
test('starting a second timer offers to switch instead of failing', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'tracker-switch');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    const idIssueFirst = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: ISSUE_DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });
    const idIssueSecond = await Interaction.createIssue(page, idProject, {
        title: OTHER_ISSUE_TITLE,
        description: ISSUE_DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    const startButton = page.locator('app-tracker').getByRole('button', { name: 'Start tracking' });
    const control = page.getByTestId('tracker-control');

    await page.goto(`/project/${idProject}/issue/${idIssueFirst}`);
    await startButton.click();
    await expect(page.getByTestId('tracker-control-issue-link')).toContainText(`#${idIssueFirst}`);
    // Let a whole second accrue: a timer submitted at 0s logs nothing to show later.
    await expect(page.getByTestId('tracker-control-clock')).toHaveText(/00:00:0[1-9]/);

    await page.goto(`/project/${idProject}/issue/${idIssueSecond}`);
    await startButton.click();

    const dialog = page.getByTestId('tracker-switch-dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('.tsd-ref')).toHaveText([`#${idIssueFirst}`, `#${idIssueSecond}`]);
    await expect(dialog.locator('.tsd-title')).toHaveText([ISSUE_TITLE, OTHER_ISSUE_TITLE]);
    await expect(dialog.getByTestId('tracker-switch-elapsed')).toHaveText(/00:00:\d{2}/);
    await expect(page.locator('.ui-toast--error')).toHaveCount(0);

    await page.getByTestId('tracker-switch-confirm').click();

    await expect(page.getByTestId('tracker-control-issue-link')).toContainText(`#${idIssueSecond}`);
    await expect(control).toBeVisible();

    await page.goto(`/project/${idProject}/issue/${idIssueFirst}`);
    await expect(page.locator('#issue-tracked')).toHaveText(/\d+s/);
    await expect(page.locator('app-activity-time-item')).toHaveCount(1);
});

// Scenario:
// - create a dedicated non-admin user through the admin API and log in
// - create a blank project and an issue in it
// - log 45m by hand through the tracker widget on the issue detail
// - assert the tracked total and the timeline entry both read 45m, with no note yet
// - hover the timeline entry and open its inline edit form
// - change the duration to 1h 30m and add a note, then save
// - assert the entry, its note and the tracked total all update without a reload
// - reload and assert the edit survived the round trip
// - delete the entry from the timeline and confirm the prompt
// - assert the entry is gone and the tracked total is back to empty
test('a track record logged by hand can be edited and deleted from the timeline', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'tracker-edit');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: ISSUE_DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    const trackedValue = page.locator('#issue-tracked');
    const widget = page.locator('app-tracker');

    await widget.locator('#issue-tracker').fill('45m');
    await widget.getByRole('button', { name: 'Save tracked time' }).click();

    await expect(trackedValue).toHaveText('45m');
    const timeItem = page.locator('app-activity-time-item');
    await expect(timeItem).toHaveCount(1);
    await expect(timeItem.locator('.duration-badge')).toHaveText('45m');
    await expect(timeItem.locator('.note-prompt')).toHaveText('Add a note');

    await timeItem.locator('.time-card').hover();
    await timeItem.getByTestId('track-edit').click();
    await timeItem.getByTestId('track-edit-duration').fill('1h 30m');
    await timeItem.getByTestId('track-edit-note').fill(NOTE);
    await timeItem.getByTestId('track-edit-save').click();

    await expect(timeItem.locator('.duration-badge')).toHaveText('1h 30m');
    await expect(timeItem.locator('.note')).toHaveText(NOTE);
    await expect(trackedValue).toHaveText('1h 30m');

    await page.reload();
    await expect(trackedValue).toHaveText('1h 30m');
    await expect(page.locator('app-activity-time-item .note')).toHaveText(NOTE);

    await page.locator('app-activity-time-item .time-card').hover();
    await page.getByTestId('track-delete').click();
    await Interaction.acceptConfirm(page);

    await expect(page.locator('app-activity-time-item')).toHaveCount(0);
    await expect(trackedValue).toHaveText('');
});
