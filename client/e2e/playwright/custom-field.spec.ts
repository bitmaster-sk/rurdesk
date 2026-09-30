import { test, expect } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';
import { CustomFieldNoticeLog } from './support/issue-notices';

const PROJECT_NAME = 'Custom Field Project';
const ISSUE_TITLE = 'Invoice export misses the tax column';
const DESCRIPTION = 'Created by the custom field e2e test.';

// Scenario:
// - create a dedicated non-admin user and log in
// - create a blank project and open its settings
// - add a select field "Customer" with two options
// - create a task and set the field to "Acme" on its detail
// - reload the detail and assert the value survived
test('a custom field value survives a reload', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'custom-field');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);

    await page.goto(`/project/${idProject}/settings`);
    await page.getByRole('button', { name: /new field/i }).click();
    await page.locator('#custom-field-name').fill('Customer');
    await Interaction.pickOption(page, '#custom-field-type', 'Select');
    await page.getByRole('button', { name: /add option/i }).click();
    await page.locator('[data-testid="custom-field-option"]').nth(0).fill('Acme');
    await page.getByRole('button', { name: /add option/i }).click();
    await page.locator('[data-testid="custom-field-option"]').nth(1).fill('Northwind');
    await page.getByRole('button', { name: 'Save', exact: true }).click();

    const fieldRows = page.locator('[data-testid="custom-field-row"]');
    await expect(fieldRows).toHaveCount(1);
    await expect(fieldRows.nth(0)).toHaveText(/customer/);

    const idIssue = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    const saved = Interaction.waitForIssueSaved(page, idProject, idIssue);
    await Interaction.pickOption(page, '#cf-customer', 'Acme');
    await expect(page.locator('#cf-customer')).toHaveText(/Acme/);
    await saved;

    await page.reload();
    await expect(page.locator('#cf-customer')).toHaveText(/Acme/);
});

// Scenario:
// - log in and create a project with a text field "note"
// - open the same task's detail in a second browser context
// - change the field value in the first context
// - assert the second context receives an issue notice carrying the new value
// - assert the second context's input shows it without a reload
test('a value change reaches an open detail over the websocket', async ({
    browser,
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'custom-field-ws');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME + ' WS');

    await page.goto(`/project/${idProject}/settings`);
    await page.getByRole('button', { name: /new field/i }).click();
    await page.locator('#custom-field-name').fill('Note');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.locator('[data-testid="custom-field-row"]')).toHaveCount(1);

    const idIssue = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    const watcher = await browser.newContext({ storageState: await page.context().storageState() });
    const watcherPage = await watcher.newPage();
    const notices = CustomFieldNoticeLog.attachIssueNotices(watcherPage);
    await watcherPage.goto(`/project/${idProject}/issue/${idIssue}`);
    await expect(watcherPage.locator('#cf-note')).toBeVisible();

    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    await page.locator('#cf-note').fill('live value');
    await page.locator('#cf-note').blur();

    await notices.waitForCustomFieldValue('note', 'live value', 10_000);
    await expect(watcherPage.locator('#cf-note')).toHaveValue('live value');
    await watcher.close();
});

// Scenario:
// - log in and create a project with a select field "customer"
// - open a task detail in a second browser context
// - rename the field in the first context
// - assert the second context receives a custom_field notice for the rename
test('a definition change reaches open clients over the websocket', async ({
    browser,
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'custom-field-def');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME + ' DEF');

    await page.goto(`/project/${idProject}/settings`);
    await page.getByRole('button', { name: /new field/i }).click();
    await page.locator('#custom-field-name').fill('Note');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.locator('[data-testid="custom-field-row"]')).toHaveCount(1);

    const idIssue = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    const watcher = await browser.newContext({ storageState: await page.context().storageState() });
    const watcherPage = await watcher.newPage();
    const notices = CustomFieldNoticeLog.attach(watcherPage);
    await watcherPage.goto(`/project/${idProject}/issue/${idIssue}`);
    await expect(watcherPage.locator('#cf-note')).toBeVisible();

    await page.goto(`/project/${idProject}/settings`);
    await page
        .locator('[data-testid="custom-field-row"]')
        .first()
        .locator('[data-testid="custom-field-edit"]')
        .click();
    await page.locator('#custom-field-name').fill('Remark');
    await page.getByRole('button', { name: 'Save', exact: true }).click();

    await notices.waitForAction('u', 10_000);
    await watcher.close();
});

// Scenario:
// - log in and create a project with a required number field "impact"
// - open the new task form and fill everything except the custom field
// - assert the field is marked required and Save stays disabled
// - fill the field, save, and assert the value is on the created task after a reload
test('a required field blocks creating a task until it is filled', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'custom-field-req');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME + ' REQ');
    await Interaction.createCustomField(page, idProject, {
        name: 'Impact',
        key: 'impact',
        type: 'Number',
        isRequired: true
    });

    await page.goto(`/project/${idProject}/issue/view/table`);
    await page.getByRole('button', { name: /add task/i }).click();
    await page.waitForURL(`**/project/${idProject}/issue/0`);

    await page.locator('input[formcontrolname="title"]').fill(ISSUE_TITLE);
    await page.locator('app-message-editor [contenteditable="true"]').fill(DESCRIPTION);
    await page.locator('input[formcontrolname="title"]').blur();
    await Interaction.pickOption(page, '#issue-state', 'New');
    await Interaction.pickOption(page, '#issue-severity', 'Medium');

    const save = page.getByRole('button', { name: 'Save', exact: true });
    await expect(page.locator('label[for="cf-impact"]')).toHaveClass(/required/);
    await expect(save).toBeDisabled();

    await page.locator('#cf-impact').fill('7');
    await page.locator('#cf-impact').blur();
    await expect(save).toBeEnabled();

    await save.click();
    await page.waitForURL(/\/issue\/[1-9]\d*$/);

    await page.reload();
    await expect(page.locator('#cf-impact')).toHaveValue('7');
});

// Scenario:
// - log in, create a project and a task while no custom field exists
// - add a required field afterwards
// - open the task and assert it explains the field became required later
// - rename the task and assert the rename still saves, so the rule never blocks old tasks
test('a task older than the rule stays editable and says why the field is empty', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'custom-field-old');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME + ' OLD');
    const idIssue = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    await Interaction.createCustomField(page, idProject, {
        name: 'Impact',
        key: 'impact',
        type: 'Number',
        isRequired: true
    });

    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    await expect(page.getByTestId('issue-custom-field-missing')).toContainText(
        /became required after/i
    );

    const saved = Interaction.waitForIssueSaved(page, idProject, idIssue);
    await page.locator('input[formcontrolname="title"]').fill('Renamed while empty');
    await page.locator('input[formcontrolname="title"]').blur();
    await saved;

    await page.reload();
    await expect(page.locator('input[formcontrolname="title"]')).toHaveValue('Renamed while empty');
    await expect(page.getByTestId('issue-custom-field-missing')).toBeVisible();
});

// Scenario:
// - log in and create a project with a required number field carrying a default
// - open the new task form and assert the field is already filled with the default
// - fill in title, description, state and severity and save
// - assert the created task kept the default after a reload
test('a default value prefills a new task and satisfies the required rule', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'custom-field-def-value');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME + ' DEFVAL');
    await Interaction.createCustomField(page, idProject, {
        name: 'Impact',
        key: 'impact',
        type: 'Number',
        isRequired: true,
        defaultValue: '7'
    });

    await page.goto(`/project/${idProject}/issue/view/table`);
    await page.getByRole('button', { name: /add task/i }).click();
    await page.waitForURL(`**/project/${idProject}/issue/0`);
    await expect(page.locator('#cf-impact')).toHaveValue('7');

    await page.locator('input[formcontrolname="title"]').fill(ISSUE_TITLE);
    await page.locator('app-message-editor [contenteditable="true"]').fill(DESCRIPTION);
    await page.locator('input[formcontrolname="title"]').blur();
    await Interaction.pickOption(page, '#issue-state', 'New');
    await Interaction.pickOption(page, '#issue-severity', 'Medium');

    const save = page.getByRole('button', { name: 'Save', exact: true });
    await expect(save).toBeEnabled();
    await save.click();
    await page.waitForURL(/\/issue\/[1-9]\d*$/);

    await page.reload();
    await expect(page.locator('#cf-impact')).toHaveValue('7');
});

// Scenario:
// - log in and create a project with one field of every type
// - create a task and give every field a value, the date one over the API
// - reload the detail while collecting JavaScript errors
// - assert every value is shown and nothing was reported: a control that loops the
//   change detection hangs the page here, and a production build logs nothing for it
test('a detail carrying every field type loads without a console error', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'custom-field-console');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME + ' CONSOLE');
    await Interaction.createCustomField(page, idProject, { name: 'Note', key: 'note' });
    await Interaction.createCustomField(page, idProject, {
        name: 'Impact',
        key: 'impact',
        type: 'Number'
    });
    await Interaction.createCustomField(page, idProject, { name: 'Due', key: 'due', type: 'Date' });
    await Interaction.createCustomField(page, idProject, {
        name: 'Reviewed',
        key: 'reviewed',
        type: 'Yes/No'
    });
    await Interaction.createCustomField(page, idProject, {
        name: 'Customer',
        key: 'customer',
        type: 'Select',
        options: ['Acme']
    });

    const idIssue = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    const token = await Interaction.apiToken(request, baseURL!, user);
    await Interaction.setCustomFieldValues(request, baseURL!, token, idProject, idIssue, {
        note: 'a note',
        impact: 3,
        due: '2026-03-04T00:00:00Z',
        reviewed: true
    });

    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    const saved = Interaction.waitForIssueSaved(page, idProject, idIssue);
    await Interaction.pickOption(page, '#cf-customer', 'Acme');
    await expect(page.locator('#cf-customer')).toHaveText(/Acme/);
    await saved;

    const problems: string[] = [];
    page.on('console', message => {
        // Only JavaScript errors: a failed request logs one of its own, and the quality
        // endpoint answers 404 until a task has a report.
        if (message.type() === 'error' && !message.text().includes('Failed to load resource')) {
            problems.push(`console: ${message.text()}`);
        }
    });
    page.on('pageerror', error => problems.push(`pageerror: ${error.message}`));

    await page.reload();
    await expect(page.locator('#cf-note')).toHaveValue('a note');
    await expect(page.locator('#cf-due')).not.toHaveValue('');
    await expect(page.locator('#cf-reviewed')).toBeChecked();
    await expect(page.locator('#cf-customer')).toHaveText(/Acme/);
    expect(problems).toEqual([]);
});

// Scenario:
// - log in and create a project with an optional text field "owner"
// - create one task with a value and one without
// - mark the field required and fill existing tasks with "unassigned" in the same step
// - assert the blank task got the filled value and the answered task kept its own
test('filling existing tasks never overwrites an answer', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'custom-field-fill');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME + ' FILL');
    await Interaction.createCustomField(page, idProject, { name: 'Owner', key: 'owner' });

    const idAnswered = await Interaction.createIssue(page, idProject, {
        title: 'Answered task',
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });
    await page.goto(`/project/${idProject}/issue/${idAnswered}`);
    await page.locator('#cf-owner').fill('Jane');
    await page.locator('#cf-owner').blur();
    await expect(page.locator('#cf-owner')).toHaveValue('Jane');

    const idBlank = await Interaction.createIssue(page, idProject, {
        title: 'Blank task',
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });

    await page.goto(`/project/${idProject}/settings`);
    await page.getByTestId('custom-field-row').first().getByTestId('custom-field-edit').click();
    await page.locator('#custom-field-required').check();
    await expect(page.getByTestId('custom-field-backfill')).toBeVisible();
    await page.locator('#custom-field-backfill').fill('unassigned');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.getByTestId('custom-field-backfill')).toBeHidden();

    await page.goto(`/project/${idProject}/issue/${idBlank}`);
    await expect(page.locator('#cf-owner')).toHaveValue('unassigned');

    await page.goto(`/project/${idProject}/issue/${idAnswered}`);
    await expect(page.locator('#cf-owner')).toHaveValue('Jane');
});

// Scenario:
// - log in and create a project, then open the new field form
// - type an accented name and assert the locked key field fills itself
// - save and assert the row and the task detail both use the derived key
// - open the form again with a name that derives the same key
// - assert the clash is named, Save is refused, and editing the key clears it
test('the key comes from the name and a clash is refused', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'custom-field-key');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME + ' KEY');

    await page.goto(`/project/${idProject}/settings`);
    await page.getByRole('button', { name: /new field/i }).click();
    await page.locator('#custom-field-name').fill('Termín dodania');
    await expect(page.locator('#custom-field-key')).toHaveValue('termin_dodania');
    await expect(page.locator('#custom-field-key')).toBeDisabled();
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(
        page.getByTestId('custom-field-row').filter({ hasText: 'termin_dodania' })
    ).toHaveCount(1);

    const idIssue = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });
    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    await expect(page.locator('#cf-termin_dodania')).toBeVisible();

    await page.goto(`/project/${idProject}/settings`);
    await page.getByRole('button', { name: /new field/i }).click();
    await page.locator('#custom-field-name').fill('Termin Dodania');
    await expect(page.getByTestId('custom-field-key-taken')).toContainText('Termín dodania');
    await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeDisabled();

    await page.locator('#custom-field-key').fill('termin_dodania_2');
    await expect(page.getByTestId('custom-field-key-taken')).toBeHidden();
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.getByTestId('custom-field-row')).toHaveCount(2);
});

// Scenario:
// - log in and create a project with a select field "Customer" holding Acme and Northwind
// - create a task and set the field to Acme
// - edit the field: add a third option Globex and remove Acme, which still has the value
// - assert the migration dialog offers Globex even though it was never saved
// - migrate to Globex and assert the task now shows Globex
test('an in-use option migrates to one added in the same step', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'custom-field-migrate');
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME + ' MIGRATE');

    await page.goto(`/project/${idProject}/settings`);
    await page.getByRole('button', { name: /new field/i }).click();
    await page.locator('#custom-field-name').fill('Customer');
    await Interaction.pickOption(page, '#custom-field-type', 'Select');
    await page.getByRole('button', { name: /add option/i }).click();
    await page.locator('[data-testid="custom-field-option"]').nth(0).fill('Acme');
    await page.getByRole('button', { name: /add option/i }).click();
    await page.locator('[data-testid="custom-field-option"]').nth(1).fill('Northwind');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.getByTestId('custom-field-row')).toHaveCount(1);

    const idIssue = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });
    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    const saved = Interaction.waitForIssueSaved(page, idProject, idIssue);
    await Interaction.pickOption(page, '#cf-customer', 'Acme');
    await saved;

    await page.goto(`/project/${idProject}/settings`);
    await page.getByTestId('custom-field-edit').click();
    await page.getByRole('button', { name: /add option/i }).click();
    await page.locator('[data-testid="custom-field-option"]').nth(2).fill('Globex');
    await page.getByTestId('custom-field-option-remove').nth(0).click();
    await page.getByRole('button', { name: 'Save', exact: true }).click();

    await Interaction.pickOption(page, '#delete-migration-target', 'Globex');
    await page.getByTestId('delete-migration-confirm').click();
    await expect(page.getByTestId('delete-migration-confirm')).toBeHidden();

    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    await expect(page.locator('#cf-customer')).toHaveText(/Globex/);
});
