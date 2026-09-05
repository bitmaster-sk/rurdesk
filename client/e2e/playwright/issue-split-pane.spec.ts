import { test, expect, Locator, Page } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Split Pane Project';
const ISSUE_TITLE = 'Resize the detail panes';
const ISSUE_DESCRIPTION = 'Created by the split pane e2e test.';

async function widthOf(pane: Locator): Promise<number> {
    const box = await pane.boundingBox();
    return box!.width;
}

function infoPane(page: Page): Locator {
    return page.getByTestId('split-pane-start');
}

function activityPane(page: Page): Locator {
    return page.getByTestId('split-pane-end');
}

async function paneWidthGap(page: Page): Promise<number> {
    return Math.abs((await widthOf(infoPane(page))) - (await widthOf(activityPane(page))));
}

// Grabs the separator near its top edge. Its centre is occupied by the collapse and
// reset buttons, which take the pointerdown themselves and cancel the drag.
const GRAB_OFFSET_Y = 20;

async function openDetail(page: Page): Promise<{ idProject: number; idIssuePublic: number }> {
    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    const idIssuePublic = await Interaction.createIssue(page, idProject, {
        title: ISSUE_TITLE,
        description: ISSUE_DESCRIPTION,
        state: 'New',
        severity: 'Medium'
    });
    await expect(page.getByTestId('split-pane-splitter')).toBeVisible();
    await Interaction.waitForStableBox(page.getByTestId('split-pane-splitter'));
    return { idProject, idIssuePublic };
}

// Scenario:
// - create a dedicated user, log in, create a project and an issue
// - open the issue detail and read the starting width of both panes
// - drag the separator to the right and assert the info pane grew while activity shrank
// - assert the two panes still fill the same total width (nothing overflows)
// - reload the page and assert the dragged ratio survived the round trip
test('dragging the separator resizes both panes and the ratio survives a reload', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'split-pane-drag');
    await Interaction.login(page, user);
    await openDetail(page);

    const infoBefore = await widthOf(infoPane(page));
    const activityBefore = await widthOf(activityPane(page));
    const totalBefore = infoBefore + activityBefore;

    await Interaction.dragBy(page, page.getByTestId('split-pane-splitter'), 200, GRAB_OFFSET_Y);

    await expect.poll(() => widthOf(infoPane(page))).toBeGreaterThan(infoBefore + 100);
    const infoAfter = await widthOf(infoPane(page));
    const activityAfter = await widthOf(activityPane(page));

    expect(activityAfter).toBeLessThan(activityBefore - 100);
    expect(infoAfter + activityAfter).toBeCloseTo(totalBefore, 0);

    await page.reload();
    await expect(page.getByTestId('split-pane-splitter')).toBeVisible();
    expect(await widthOf(infoPane(page))).toBeCloseTo(infoAfter, 0);
});

// Scenario:
// - create a dedicated user, log in, create a project and an issue
// - open the issue detail and hover the separator to reveal its controls
// - click the "collapse left" control and assert the info pane is gone while activity fills the row
// - assert the collapse control is now disabled and the restore control is not
// - click the opposite control and assert the info pane is back and readable
// - click the "collapse right" control and assert the activity pane is gone instead
test('the separator controls collapse either pane and bring it back', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'split-pane-collapse');
    await Interaction.login(page, user);
    await openDetail(page);

    const collapseStart = page.getByTestId('split-pane-collapse-start');
    const collapseEnd = page.getByTestId('split-pane-collapse-end');
    const totalBefore = (await widthOf(infoPane(page))) + (await widthOf(activityPane(page)));

    // The controls are revealed by hovering the separator, so a real user hovers first.
    await page.getByTestId('split-pane-splitter').hover();
    await collapseStart.click();
    await expect.poll(() => widthOf(infoPane(page))).toBe(0);
    expect(await widthOf(activityPane(page))).toBeGreaterThan(totalBefore - 20);
    await expect(collapseStart).toBeDisabled();
    await expect(collapseEnd).toBeEnabled();

    await collapseEnd.click();
    await expect.poll(() => widthOf(infoPane(page))).toBeGreaterThan(0);
    await expect(page.locator('input[formcontrolname="title"]')).toHaveValue(ISSUE_TITLE);

    await collapseEnd.click();
    await expect.poll(() => widthOf(activityPane(page))).toBe(0);
    expect(await widthOf(infoPane(page))).toBeGreaterThan(totalBefore - 20);
    await expect(collapseEnd).toBeDisabled();
});

// Scenario:
// - create a dedicated user, log in, create a project and an issue
// - open the issue detail and drag the separator far past the left edge
// - assert the info pane stopped at its minimum width instead of disappearing
// - double-click the separator and assert both panes are back to an even split
test('the separator refuses to drag a pane below its minimum width', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'split-pane-min');
    await Interaction.login(page, user);
    await openDetail(page);

    const splitter = page.getByTestId('split-pane-splitter');
    const infoBefore = await widthOf(infoPane(page));
    await Interaction.dragBy(page, splitter, -2000, GRAB_OFFSET_Y);

    await expect.poll(() => widthOf(infoPane(page))).toBeLessThan(infoBefore - 100);
    expect(await widthOf(infoPane(page))).toBeGreaterThanOrEqual(279);

    const box = (await splitter.boundingBox())!;
    await splitter.dblclick({ position: { x: box.width / 2, y: GRAB_OFFSET_Y } });

    await expect.poll(() => paneWidthGap(page)).toBeLessThan(20);
});

// Scenario:
// - create a dedicated user, log in, create a project and an issue
// - open the issue detail and assert the reset control is offered but inactive at the default split
// - collapse the activity pane, then hit reset and assert both panes are back and even
// - reload the page and assert the reset stuck rather than resurrecting the old split
test('the reset control returns the panes to the default split', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'split-pane-reset');
    await Interaction.login(page, user);
    await openDetail(page);

    const reset = page.getByTestId('split-pane-reset');
    await page.getByTestId('split-pane-splitter').hover();
    await expect(reset).toBeDisabled();

    await page.getByTestId('split-pane-collapse-end').click();
    expect(await widthOf(activityPane(page))).toBe(0);
    await expect(reset).toBeEnabled();

    await reset.click();
    const info = await widthOf(infoPane(page));
    const activity = await widthOf(activityPane(page));
    expect(Math.abs(info - activity)).toBeLessThan(20);
    await expect(reset).toBeDisabled();

    await page.reload();
    await expect(page.getByTestId('split-pane-splitter')).toBeVisible();
    expect(await widthOf(activityPane(page))).toBeGreaterThan(0);
    expect(Math.abs((await widthOf(infoPane(page))) - activity)).toBeLessThan(20);
});
