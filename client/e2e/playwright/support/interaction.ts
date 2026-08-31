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
        await page.waitForURL(url => !url.pathname.endsWith('/login'));
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

    public static async createUserApiKey(page: Page, name: string): Promise<string> {
        await page.getByTestId('user-api-key-name').fill(name);
        await page.getByTestId('user-api-key-create').click();

        const revealed = page.getByTestId('user-api-key-revealed');
        await expect(revealed).toBeVisible();
        return (await revealed.locator('code').innerText()).trim();
    }

    public static async acceptConfirm(page: Page): Promise<void> {
        await page.locator('.ui-confirm-panel').getByRole('button', { name: 'Yes' }).click();
    }

    public static async openPalette(page: Page): Promise<Locator> {
        await page.keyboard.press('Control+k');
        const palette = page.locator('.palette');
        await expect(palette).toBeVisible();
        return palette;
    }
}
