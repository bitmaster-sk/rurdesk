import { test, expect, Page } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Palette Project';
const ISSUE_TITLE = 'Rebuild the search index';

// Every test here needs the same expensive precondition — a logged-in user with a
// project that holds one issue — so the suite builds it once and shares the page.
test.describe.configure({ mode: 'serial' });

test.describe('command palette', () => {
    let page: Page;
    let idProject: number;
    let idIssuePublic: number;

    test.beforeAll(async ({ browser }, testInfo) => {
        const baseURL = testInfo.project.use.baseURL!;
        page = await browser.newPage({ baseURL });

        const user = await createUser(page.request, baseURL, 'palette');
        await Interaction.login(page, user);
        idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
        idIssuePublic = await Interaction.createIssue(page, idProject, {
            title: ISSUE_TITLE,
            description: 'Created by the command palette e2e test.',
            state: 'New',
            severity: 'High'
        });
    });

    test.afterAll(async () => {
        await page.close();
    });

    // The palette primes its issue cache once, on open, and only if the page has
    // already published idProject into the command context. Waiting for the row
    // guarantees the table has loaded and done that.
    async function openTable(): Promise<void> {
        await page.goto(`/project/${idProject}/issue/view/table`);
        await expect(page.getByRole('link', { name: ISSUE_TITLE, exact: true })).toBeVisible();
    }

    // Scenario:
    // - open the issue table of a project that holds one issue
    // - press Ctrl+K and assert the palette opens focused, in "All" mode
    // - assert All mode really is unfiltered: a navigation destination AND the
    //   issue jump command from two different providers are both listed
    // - press Escape on an empty query and assert the palette is gone
    test('⌘/Ctrl+K opens the palette in All mode listing every provider', async () => {
        await openTable();

        const palette = await Interaction.openPalette(page);
        await expect(palette.locator('input')).toBeFocused();
        await expect(palette.locator('.palette__mode')).toContainText(/all/i);

        await expect(palette.locator('[data-item="nav.table"]')).toBeVisible();
        await expect(palette.locator(`[data-item="issue.jump.${idIssuePublic}"]`)).toBeVisible();

        await page.keyboard.press('Escape');
        await expect(palette).toHaveCount(0);
    });

    // Scenario:
    // - open the issue table and blur the focused element so bare keys are not typed into it
    // - press "/" and assert the palette opens with the Navigation mode chip
    // - assert the list is really filtered: navigation destinations remain, issue
    //   jump commands are gone
    // - press Escape twice: the first clears the "/" query, the second closes the palette
    test('bare "/" filters the palette down to navigation destinations', async () => {
        await openTable();
        await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());

        await page.keyboard.press('/');
        const palette = page.locator('.palette');
        await expect(palette).toBeVisible();
        await expect(palette.locator('.palette__mode')).toContainText(/navigat/i);

        await expect(palette.locator('[data-item="nav.table"]')).toBeVisible();
        await expect(palette.locator('[data-item^="issue.jump."]')).toHaveCount(0);

        await page.keyboard.press('Escape');
        await page.keyboard.press('Escape');
        await expect(palette).toHaveCount(0);
    });

    // Scenario:
    // - open the issue table and open the palette with Ctrl+K
    // - type the "#" prefix and assert the mode chip switches to Tasks
    // - assert the list is really filtered the other way round: the issue jump
    //   command remains, navigation destinations are gone
    test('"#" filters the palette down to tasks', async () => {
        await openTable();

        const palette = await Interaction.openPalette(page);
        await palette.locator('input').fill('#');

        await expect(palette.locator('.palette__mode')).toContainText(/task/i);
        await expect(palette.locator(`[data-item="issue.jump.${idIssuePublic}"]`)).toBeVisible();
        await expect(palette.locator('[data-item^="nav."]')).toHaveCount(0);

        await page.keyboard.press('Escape');
        await page.keyboard.press('Escape');
        await expect(palette).toHaveCount(0);
    });

    // Scenario:
    // - open the issue table and blur the focused element
    // - press "?" and assert the keyboard-shortcuts help sheet renders with key hints
    // - press Escape and assert the sheet is gone
    test('"?" opens the keyboard-shortcuts help sheet', async () => {
        await openTable();
        await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());

        await page.keyboard.press('?');
        const help = page.locator('[data-help]');
        await expect(help).toBeVisible();
        await expect(help.locator('kbd').first()).toBeVisible();

        await page.keyboard.press('Escape');
        await expect(help).toHaveCount(0);
    });

    // Scenario:
    // - open the issue table and open the palette with Ctrl+K
    // - type a fragment of the issue title
    // - assert the matching jump command is listed and no longer competes with the rest
    // - press Enter and assert the browser navigated to that issue's detail route
    test('searching for an issue jumps to it', async () => {
        await openTable();

        const palette = await Interaction.openPalette(page);
        await palette.locator('input').fill('rebuild');

        const jump = palette.locator(`[data-item="issue.jump.${idIssuePublic}"]`);
        await expect(jump).toBeVisible();
        await expect(jump).toContainText(ISSUE_TITLE);
        await expect(palette.locator('[data-item^="nav."]')).toHaveCount(0);

        await page.keyboard.press('Enter');
        await page.waitForURL(`**/project/${idProject}/issue/${idIssuePublic}`);
        await expect(page.locator('input[formcontrolname="title"]')).toHaveValue(ISSUE_TITLE);
    });
});
