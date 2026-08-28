import { test, expect, Locator, Page } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Focus Ring Project';
const CHROME_NATIVE_BLUE = 'rgb(0, 95, 204)';

interface FocusPaint {
    outlineStyle: string;
    outlineWidth: string;
    outlineColor: string;
    outlineOffset: string;
    boxShadow: string;
    borderColor: string;
    borderRadius: string;
    width: number;
    height: number;
}

async function paintOf(target: Locator): Promise<FocusPaint> {
    return target.evaluate(el => {
        const s = getComputedStyle(el);
        return {
            outlineStyle: s.outlineStyle,
            outlineWidth: s.outlineWidth,
            outlineColor: s.outlineColor,
            outlineOffset: s.outlineOffset,
            boxShadow: s.boxShadow,
            borderColor: s.borderColor,
            borderRadius: s.borderRadius,
            width: el.getBoundingClientRect().width,
            height: el.getBoundingClientRect().height
        };
    });
}

async function brandColor(page: Page): Promise<string> {
    return page.evaluate(() => {
        const probe = document.createElement('span');
        probe.style.color = getComputedStyle(document.documentElement).getPropertyValue(
            '--ui-focus-ring-color'
        );
        document.body.appendChild(probe);
        const resolved = getComputedStyle(probe).color;
        probe.remove();
        return resolved;
    });
}

async function tabTo(page: Page, target: Locator, maxPresses = 40): Promise<void> {
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    for (let i = 0; i < maxPresses; i++) {
        await page.keyboard.press('Tab');
        if (await target.evaluate(el => el === document.activeElement)) return;
    }
    throw new Error(`element never received focus within ${maxPresses} Tab presses`);
}

async function openProject(page: Page): Promise<number> {
    return Interaction.createBlankProject(page, PROJECT_NAME);
}

// Scenario:
// - create a dedicated user, log in and create a project so the sidebar and top menu render
// - resolve the brand focus colour from the design token, so the test tracks the token not a literal
// - tab to the sidebar overview link and assert it draws a solid brand outline
// - assert that outline is not Chrome's native blue, which is the defect being fixed
// - tab to the avatar trigger and assert it is ringed too, since it strips all native chrome
// - assert that ring is round and centred, matching the circular avatar it wraps
test('keyboard focus rings elements that carry no focus style of their own', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'focus-ring-orphans');
    await Interaction.login(page, user);
    await openProject(page);

    const brand = await brandColor(page);

    const sidebarLink = page.getByTestId('sidebar-link-overview');
    await expect(sidebarLink).toBeVisible();
    await tabTo(page, sidebarLink);

    const link = await paintOf(sidebarLink);
    expect(link.outlineStyle).toBe('solid');
    expect(parseFloat(link.outlineWidth)).toBeGreaterThanOrEqual(2);
    expect(link.outlineColor).toBe(brand);
    expect(link.outlineColor).not.toBe(CHROME_NATIVE_BLUE);

    const avatar = page.getByTestId('avatar-trigger');
    await expect(avatar).toBeVisible();
    await tabTo(page, avatar);

    const trigger = await paintOf(avatar);
    expect(trigger.outlineStyle).toBe('solid');
    expect(trigger.outlineColor).toBe(brand);
    expect(trigger.outlineColor).not.toBe(CHROME_NATIVE_BLUE);

    expect(parseFloat(trigger.borderRadius)).toBeGreaterThanOrEqual(trigger.height / 2);
    expect(Math.abs(trigger.width - trigger.height)).toBeLessThan(1);
});

// Scenario:
// - create a dedicated user, log in and create a project
// - click the sidebar link with the mouse and assert no ring is painted
// - the rule is :focus-visible, so a pointer click must stay quiet even though the element is focused
test('a mouse click focuses without painting a ring', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'focus-ring-mouse');
    await Interaction.login(page, user);
    await openProject(page);

    const sidebarLink = page.getByTestId('sidebar-link-overview');
    await sidebarLink.click();
    await expect(sidebarLink).toBeFocused();

    const paint = await paintOf(sidebarLink);
    expect(paint.outlineStyle === 'none' || parseFloat(paint.outlineWidth) === 0).toBe(true);
});

// Scenario:
// - create a dedicated user, log in, create a project and an issue, open its detail
// - tab to the collapsible participants panel header, which uiActivatable makes reachable
// - assert its ring is drawn fully, not clipped to a single line by the panel's
//   overflow: hidden — an outward ring on a flush header survives only on one edge
test('a panel header flush inside a clipping panel still shows a whole ring', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'focus-ring-panel');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
    await Interaction.createIssue(page, idProject, {
        title: 'Panel focus ring',
        description: 'Created by the focus ring e2e test.',
        state: 'New',
        severity: 'Medium'
    });

    const header = page.locator('.participants-panel__header');
    await expect(header).toBeVisible();
    await tabTo(page, header, 60);

    const paint = await paintOf(header);
    expect(paint.outlineStyle).toBe('solid');
    expect(paint.outlineColor).toBe(await brandColor(page));

    expect(parseFloat(paint.outlineOffset)).toBeLessThan(0);
});

// Scenario:
// - create a dedicated user, log in, create a project and open the task table
// - tab across the toolbar collecting every element that takes focus
// - assert the "add task" control is reached, so the button is still keyboard-operable
// - assert no <ui-button> host ever took focus: RouterLink puts tabindex="0" on a
//   non-anchor host, which used to give that button two stops and ring the outer,
//   unrounded wrapper on the first one
test('a ui-button with a routerLink is a single tab stop on the button itself', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'focus-ring-tabstops');
    await Interaction.login(page, user);
    const idProject = await openProject(page);

    await page.goto(`/project/${idProject}/issue/view/table`);
    const addTask = page.getByRole('button', { name: /add task/i });
    await expect(addTask).toBeVisible();

    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    const visited: string[] = [];
    let reachedAddTask = false;
    for (let i = 0; i < 30 && !reachedAddTask; i++) {
        await page.keyboard.press('Tab');
        visited.push(await page.evaluate(() => document.activeElement?.tagName ?? 'NONE'));
        reachedAddTask = await addTask.evaluate(el => el === document.activeElement);
    }

    expect(reachedAddTask).toBe(true);
    expect(visited).not.toContain('UI-BUTTON');
});

// Scenario:
// - create a dedicated user, log in and create a project, then open the new-issue form
// - tab to the title input and assert it keeps the halo shape: brand border plus a box-shadow aura
// - assert it did NOT also pick up the global outline, so field controls show one shape and not two
test('field controls keep the halo shape instead of gaining a second ring', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'focus-ring-fields');
    await Interaction.login(page, user);
    const idProject = await openProject(page);

    await page.goto(`/project/${idProject}/issue/view/table`);
    await page.getByRole('button', { name: /add task/i }).click();
    await page.waitForURL(`**/project/${idProject}/issue/0`);

    const title = page.locator('input[formcontrolname="title"]');
    await expect(title).toBeVisible();
    await tabTo(page, title);

    const brand = await brandColor(page);

    await expect.poll(async () => (await paintOf(title)).borderColor).toBe(brand);

    const paint = await paintOf(title);
    expect(paint.boxShadow).not.toBe('none');
    expect(paint.outlineStyle === 'none' || parseFloat(paint.outlineWidth) === 0).toBe(true);
});
