import { APIRequestContext, expect, test } from '@playwright/test';
import { Interaction } from './support/interaction';
import { authHeaders } from './support/auth';
import { tokenOf } from './support/sprint';
import { createUser } from './support/user';

async function createProject(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    name: string
): Promise<number> {
    const res = await request.post(`${baseURL}/api/private/project`, {
        headers: authHeaders(token),
        data: { name, color: '#123456' }
    });
    expect(res.status(), await res.text()).toBe(200);
    return ((await res.json()) as { idProject: number }).idProject;
}

// Scenario:
// - create a dedicated user, log in and create a blank project
// - fold the side menu with its toggle and assert it is a narrow rail whose links keep their names
// - reload and assert the rail stays
// - press "[" and assert the menu unfolds again
test('the side menu folds into a rail that survives a reload', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'nav-rail');
    await Interaction.login(page, user);
    await Interaction.createBlankProject(page, 'Rail Project');
    const menu = page.getByTestId('project-left-menu');
    const board = page.getByTestId('sidebar-link-kanban');
    const wide = (await menu.boundingBox())!.width;

    await page.getByTestId('side-menu-toggle').click();

    await expect.poll(async () => (await menu.boundingBox())!.width).toBeLessThan(wide / 2);
    await expect(board).toHaveAccessibleName('Board');

    await page.reload();
    await expect(board).toBeVisible();
    expect((await menu.boundingBox())!.width).toBeLessThan(wide / 2);

    await page.keyboard.press('[');
    await expect.poll(async () => (await menu.boundingBox())!.width).toBeCloseTo(wide, 0);
});

// Scenario:
// - create a dedicated user with two projects and open the board of the first one
// - press "p", pick the second project and assert the board of the second project opens
// - open Agent chat, pick the first project and assert its overview opens
test('switching project keeps the section, and a global page lands on the overview', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'nav-switch');
    await Interaction.login(page, user);
    const idFirst = await Interaction.createBlankProject(page, 'Switch First');
    const token = await tokenOf(request, baseURL!, user);
    const idSecond = await createProject(request, baseURL!, token, 'Switch Second');

    await page.goto(`/project/${idFirst}/issue/view/kanban`);
    await expect(page.getByTestId('project-switcher')).toContainText('Switch First');
    await page.keyboard.press('p');
    await page.getByTestId('project-switcher-option').filter({ hasText: 'Switch Second' }).click();
    await page.waitForURL(`**/project/${idSecond}/issue/view/kanban`);
    await expect(page.getByTestId('project-switcher')).toContainText('Switch Second');

    await page.getByTestId('agent-chat-menu').click();
    await page.waitForURL(/\/agent-chat/);
    await page.getByTestId('project-switcher').click();
    await page.getByTestId('project-switcher-option').filter({ hasText: 'Switch First' }).click();
    await page.waitForURL(`**/project/${idFirst}/view`);
});

// Scenario:
// - create a dedicated user, log in and create a blank project
// - open Messages from the side menu and hide its panel
// - open Agent chat and assert its panel is hidden too, with the show button in its place
// - open the wiki, assert the same, and show the panel from there
// - reload and open Messages, assert its panel is shown again
test('the secondary panel hides and shows together on Wiki, Messages and Agent chat', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'nav-panel');
    await Interaction.login(page, user);
    await Interaction.createBlankProject(page, 'Panel Project');
    const panel = page.locator('ui-side-panel');
    const show = page.getByTestId('side-panel-show');

    await page.getByTestId('sidebar-messages').click();
    await page.waitForURL(/\/message\/\d+\/\d+\/view$/);
    await expect(page.getByTestId('message-conversation').first()).toBeVisible();
    await page.getByTestId('side-panel-hide').click();
    await expect(panel).toHaveClass(/ui-side-panel--hidden/);

    await page.getByTestId('agent-chat-menu').click();
    await page.waitForURL(/\/agent-chat/);
    await expect(panel).toHaveClass(/ui-side-panel--hidden/);
    await expect(show).toBeVisible();

    await page.getByTestId('sidebar-link-wiki').click();
    await page.waitForURL(/\/wiki/);
    await expect(panel).toHaveClass(/ui-side-panel--hidden/);
    await show.click();
    await expect(panel).toBeVisible();
    await expect(show).toBeHidden();

    await page.reload();
    await page.getByTestId('sidebar-messages').click();
    await page.waitForURL(/\/message\/\d+\/\d+\/view$/);
    await expect(panel).toBeVisible();
    await expect(page.getByTestId('message-conversation').first()).toBeVisible();
});
