import { test, expect, Locator, Page } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';
import { stateIdOf } from './support/sprint';

const TITLES = ['Fix the login redirect', 'Cache the avatar images', 'Write the release notes'];

abstract class BulkEdit {
    public static tableRow(page: Page, title: string): Locator {
        return page.locator('tbody tr', {
            has: page.getByRole('link', { name: title, exact: true })
        });
    }

    public static tile(page: Page, title: string): Locator {
        return page.locator('app-issue-kanban-tile', { hasText: title });
    }

    public static async reviewAndApply(page: Page, expectedSummary: string[]): Promise<void> {
        await page.getByTestId('bulk-review').click();
        const summary = page.getByTestId('bulk-summary');
        for (const text of expectedSummary) await expect(summary).toContainText(text);
        await page.getByTestId('bulk-apply').click();
        await expect(summary).toHaveCount(0);
    }
}

// Scenario:
// - create a dedicated non-admin user, log in and create a blank project
// - create the tag "next" and three New tasks through the API
// - open the table and Cmd/Ctrl+click two of the rows
// - assert both rows are selected and the toolbar shows "Bulk edit (2)"
// - open the dialog, pick the state In progress and click the "next" tag once to add it
// - review the changes, assert the summary lists In progress and next, and apply
// - assert the two edited rows show In progress and the tag
// - assert the third row is still New and has no tags
test('bulk edit changes the state and adds a tag on the selected table rows', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'bulk-table');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Bulk Table Project');
    const token = await Interaction.apiToken(request, baseURL!, user);
    const idNew = await stateIdOf(request, baseURL!, token, idProject, 'New');
    await Interaction.createTagViaApi(request, baseURL!, token, idProject, 'next');
    await Interaction.createIssuesViaApi(request, baseURL!, token, idProject, idNew, TITLES);

    await page.goto(`/project/${idProject}/issue/view/table`);
    const [first, second, untouched] = TITLES.map(title => BulkEdit.tableRow(page, title));
    await expect(untouched).toBeVisible();
    await first.locator('td.id-col').click({ modifiers: ['ControlOrMeta'] });
    await second.locator('td.id-col').click({ modifiers: ['ControlOrMeta'] });
    await expect(first).toHaveAttribute('aria-selected', 'true');
    await expect(second).toHaveAttribute('aria-selected', 'true');
    await expect(untouched).toHaveAttribute('aria-selected', 'false');

    const open = page.getByTestId('bulk-edit-open');
    await expect(open).toContainText('Bulk edit (2)');
    await open.click();
    await Interaction.pickOption(page, '#bulk-state', 'In progress');
    const tag = page.getByRole('checkbox', { name: /next/ });
    await tag.click();
    await expect(tag).toHaveAttribute('aria-checked', 'true');
    await BulkEdit.reviewAndApply(page, ['In progress', 'next']);

    for (const row of [first, second]) {
        await expect(row).toContainText('In progress');
        await expect(row.getByTestId('issue-tags')).toContainText('next');
    }
    await expect(untouched).toContainText('New');
    await expect(untouched.getByTestId('issue-tags')).toHaveCount(0);
});

// Scenario:
// - create a dedicated non-admin user, log in and create a blank project
// - create three New tasks through the API, all assigned to that user
// - open the board, press j to highlight the first card and x to select it
// - press Shift+ArrowDown to extend the selection to the next card in the column
// - assert exactly two cards are selected and remember their titles
// - press e to open the bulk edit dialog
// - pick Unassigned at the top of the assignee list, review and apply
// - assert the two selected cards show no assignee
// - assert the third card is still assigned
test('the board selects cards with the keyboard and bulk unassigns them', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'bulk-board');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Bulk Board Project');
    const token = await Interaction.apiToken(request, baseURL!, user);
    const idNew = await stateIdOf(request, baseURL!, token, idProject, 'New');
    const idUser = await Interaction.currentUserId(request, baseURL!, token);
    await Interaction.createIssuesViaApi(
        request,
        baseURL!,
        token,
        idProject,
        idNew,
        TITLES,
        idUser
    );

    await page.goto(`/project/${idProject}/issue/view/kanban`);
    for (const title of TITLES) await expect(BulkEdit.tile(page, title)).toBeVisible();

    await page.keyboard.press('j');
    await page.keyboard.press('x');
    await page.keyboard.press('Shift+ArrowDown');
    const selected = page.locator('app-issue-kanban-tile[aria-selected="true"]');
    await expect(selected).toHaveCount(2);
    const picked = (await selected.locator('.tile-card--title').allInnerTexts()).map(text =>
        text.trim()
    );
    const untouched = TITLES.find(title => !picked.includes(title))!;

    await page.keyboard.press('e');
    await page.getByTestId('bulk-assignee').getByRole('combobox').click();
    await page.getByRole('option').filter({ hasText: 'Unassigned' }).click();
    await BulkEdit.reviewAndApply(page, ['Unassigned']);

    for (const title of picked) {
        await expect(
            BulkEdit.tile(page, title).locator('.tile-card--avatar-unassigned')
        ).toBeVisible();
    }
    await expect(
        BulkEdit.tile(page, untouched).locator('.tile-card--avatar-unassigned')
    ).toHaveCount(0);
});
