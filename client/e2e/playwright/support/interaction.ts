import { expect, Locator, Page } from '@playwright/test';
import { TestUser } from './user';

export interface IssueDraft {
    title: string;
    description: string;
    state: string;
    severity: string;
    issueType?: string;
}

export abstract class Interaction {
    public static async login(page: Page, user: TestUser): Promise<void> {
        await page.goto('/login');
        await page.locator('input[formcontrolname="email"]').fill(user.email);
        await page.locator('input[formcontrolname="password"]').fill(user.password);
        await page.getByRole('button', { name: /login/i }).click();
    }

    public static async createBlankProject(page: Page, name: string): Promise<number> {
        await page.locator('#ob-name').fill(name);
        await page.getByRole('button', { name: /create blank project/i }).click();

        await page.waitForURL(/\/project\/\d+\/view$/);
        const idProject = Number(page.url().match(/\/project\/(\d+)\//)![1]);
        expect(idProject).toBeGreaterThan(0);
        return idProject;
    }

    public static async createIssue(
        page: Page,
        idProject: number,
        issue: IssueDraft
    ): Promise<number> {
        await page.goto(`/project/${idProject}/issue/view/table`);
        await page.getByRole('button', { name: /add task/i }).click();
        await page.waitForURL(`**/project/${idProject}/issue/0`);

        await page.locator('input[formcontrolname="title"]').fill(issue.title);
        await page.locator('app-message-editor [contenteditable="true"]').fill(issue.description);
        await page.locator('input[formcontrolname="title"]').blur();
        await Interaction.pickOption(page, '#issue-state', issue.state);
        await Interaction.pickOption(page, '#issue-severity', issue.severity);
        if (issue.issueType) {
            await Interaction.pickOption(page, '#issue-type', issue.issueType);
        }

        await page.getByRole('button', { name: 'Save', exact: true }).click();
        await page.waitForURL(/\/issue\/[1-9]\d*$/);
        return Number(page.url().match(/\/issue\/(\d+)$/)![1]);
    }

    public static async pickOption(
        page: Page,
        triggerSelector: string,
        optionName: string
    ): Promise<void> {
        await page.locator(triggerSelector).click();
        await page.getByRole('option', { name: optionName, exact: true }).click();
    }

    public static async waitForStableBox(target: Locator): Promise<void> {
        let previous = '';
        await expect
            .poll(async () => {
                const current = JSON.stringify(await target.boundingBox());
                const isStable = current === previous;
                previous = current;
                return isStable;
            })
            .toBe(true);
    }

    public static async dragBy(page: Page, handle: Locator, deltaX: number): Promise<void> {
        await Interaction.waitForStableBox(handle);
        const box = (await handle.boundingBox())!;
        const startX = box.x + box.width / 2;
        const y = box.y + box.height / 2;

        await page.mouse.move(startX, y);
        await page.mouse.down();
        // A first small move before the real one: the handle starts dragging on a
        // pointermove, and a single jump can be delivered before capture is set up.
        await page.mouse.move(startX + Math.sign(deltaX), y);
        await page.mouse.move(Math.max(0, startX + deltaX), y, { steps: 10 });
        await page.mouse.up();
    }

    public static async openPalette(page: Page): Promise<Locator> {
        await page.keyboard.press('Control+k');
        const palette = page.locator('.palette');
        await expect(palette).toBeVisible();
        return palette;
    }
}
