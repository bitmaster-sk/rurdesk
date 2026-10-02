import { test, expect, Page } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';
import { stateIdOf } from './support/sprint';

const PROJECT_NAME = 'List State Project';

function tableRows(page: Page) {
    return page.locator('tbody tr', { has: page.getByRole('link') });
}

// Scenario:
// - create a dedicated non-admin user and log in
// - create a blank project and three tasks through the API, two of them about "Login"
// - open the table, reveal the filter panel and filter the title by "Login"
// - assert only the two login tasks are listed
// - open one of them and come back with the browser back button
// - assert the title filter still reads "Login" and still lists two tasks
// - switch to the board and back to the table through the sidebar
// - assert the table still lists only the two login tasks
// - reload the page
// - assert the filter survived the reload
test('the table keeps its filter across the back button, the sidebar and a reload', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'list-filter');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    const token = await Interaction.apiToken(request, baseURL!, user);
    const idNew = await stateIdOf(request, baseURL!, token, idProject, 'New');
    await Interaction.createIssuesViaApi(request, baseURL!, token, idProject, idNew, [
        'Login fails on Safari',
        'Login times out',
        'Export the timesheet'
    ]);

    await page.goto(`/project/${idProject}/issue/view/table`);
    await expect(tableRows(page)).toHaveCount(3);
    await page.getByRole('button', { name: 'Filter', exact: true }).click();
    await page.locator('#title-filter').fill('Login');
    await expect(tableRows(page)).toHaveCount(2);

    await page.getByRole('link', { name: 'Login times out', exact: true }).click();
    await page.waitForURL(/\/issue\/\d+$/);
    await page.goBack();

    await expect(page.locator('#title-filter')).toHaveValue('Login');
    await expect(tableRows(page)).toHaveCount(2);

    await page.getByTestId('sidebar-link-kanban').click();
    await page.waitForURL(/\/issue\/view\/kanban/);
    await page.getByTestId('sidebar-link-table').click();
    await page.waitForURL(/\/issue\/view\/table/);
    await expect(tableRows(page)).toHaveCount(2);
    await expect(tableRows(page).first()).toContainText('Login');

    await page.reload();
    await expect(tableRows(page)).toHaveCount(2);
    await expect(page.getByRole('link', { name: 'Export the timesheet' })).toHaveCount(0);
});

// Scenario:
// - create a dedicated non-admin user and log in
// - create a blank project and 60 tasks through the API (more than one table page of 50)
// - open the table and assert it shows 50 of 60
// - load the second page and assert it shows 60 of 60
// - scroll to the oldest task at the bottom and open it
// - come back with the browser back button
// - assert all 60 tasks are still loaded and the opened task is back on screen
// - leave for the board and return to the table through the sidebar
// - assert the table starts again from the first page
test('back from a task brings back every loaded row and the scroll position', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'list-position');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    const token = await Interaction.apiToken(request, baseURL!, user);
    const idNew = await stateIdOf(request, baseURL!, token, idProject, 'New');
    const titles = Array.from(
        { length: 60 },
        (_, i) => `Paged task ${String(i + 1).padStart(2, '0')}`
    );
    await Interaction.createIssuesViaApi(request, baseURL!, token, idProject, idNew, titles);

    await page.goto(`/project/${idProject}/issue/view/table`);
    const count = page.getByTestId('issue-table-count');
    await expect(count).toHaveText('50 / 60');
    await page.getByRole('button', { name: 'Load more' }).click();
    await expect(count).toHaveText('60 / 60');

    const oldest = page.getByRole('link', { name: 'Paged task 01', exact: true });
    await oldest.scrollIntoViewIfNeeded();
    await oldest.click();
    await page.waitForURL(/\/issue\/\d+$/);
    await page.goBack();

    await expect(count).toHaveText('60 / 60');
    await expect(oldest).toBeInViewport();

    await page.getByTestId('sidebar-link-kanban').click();
    await page.waitForURL(/\/issue\/view\/kanban/);
    await page.getByTestId('sidebar-link-table').click();
    await expect(count).toHaveText('50 / 60');
});

// Scenario:
// - create a dedicated non-admin user and log in
// - create a blank project and two tasks through the API
// - open the board, reveal the filter panel and filter the title by "Invoice"
// - assert only the invoice task is on the board
// - open it and come back with the browser back button
// - assert the board still shows only the invoice task
test('the board keeps its filter when coming back from a task', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'board-filter');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    const token = await Interaction.apiToken(request, baseURL!, user);
    const idNew = await stateIdOf(request, baseURL!, token, idProject, 'New');
    await Interaction.createIssuesViaApi(request, baseURL!, token, idProject, idNew, [
        'Invoice totals are rounded',
        'Dark mode flickers'
    ]);

    await page.goto(`/project/${idProject}/issue/view/kanban`);
    const invoice = page.getByRole('link', { name: 'Invoice totals are rounded' });
    const darkMode = page.getByRole('link', { name: 'Dark mode flickers' });
    await expect(darkMode).toBeVisible();
    await page.getByRole('button', { name: 'Filter', exact: true }).click();
    await page.locator('#title-filter').fill('Invoice');
    await expect(darkMode).toHaveCount(0);

    await invoice.click();
    await page.waitForURL(/\/issue\/\d+$/);
    await page.goBack();

    await expect(invoice).toBeVisible();
    await expect(darkMode).toHaveCount(0);
});
