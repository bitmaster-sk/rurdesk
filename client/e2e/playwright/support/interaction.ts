import { APIRequestContext, expect, Locator, Page } from '@playwright/test';
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

    public static async createCustomField(
        page: Page,
        idProject: number,
        field: {
            name: string;
            /** Left out, the form derives it from the name. */
            key?: string;
            type?: string;
            isRequired?: boolean;
            defaultValue?: string;
            options?: string[];
        }
    ): Promise<void> {
        await page.goto(`/project/${idProject}/settings`);
        await page.getByRole('button', { name: /new field/i }).click();
        await page.locator('#custom-field-name').fill(field.name);
        if (field.key !== undefined) {
            await page.getByTestId('custom-field-key-edit').click();
            await page.locator('#custom-field-key').fill(field.key);
        }
        if (field.type) {
            await Interaction.pickOption(page, '#custom-field-type', field.type);
        }
        if (field.isRequired) {
            await page.locator('#custom-field-required').check();
        }
        if (field.defaultValue !== undefined) {
            await page.locator('#custom-field-default').fill(field.defaultValue);
        }
        for (const [index, label] of (field.options ?? []).entries()) {
            await page.getByRole('button', { name: /add option/i }).click();
            await page.getByTestId('custom-field-option').nth(index).fill(label);
        }

        await page.getByRole('button', { name: 'Save', exact: true }).click();
        // Wait on the field's own row: a count taken while the list loads can be zero.
        await expect(
            page.getByTestId('custom-field-row').filter({ hasText: field.key ?? field.name })
        ).toHaveCount(1);
    }

    // A detail autosaves on change, so a reload right after typing can cancel the
    // request that is still in flight.
    public static waitForIssueSaved(
        page: Page,
        idProject: number,
        idIssue: number
    ): Promise<unknown> {
        return page.waitForResponse(
            response =>
                response.request().method() === 'PATCH' &&
                response.url().endsWith(`/api/private/project/${idProject}/issue/${idIssue}`) &&
                response.ok()
        );
    }

    public static async apiToken(
        request: APIRequestContext,
        baseURL: string,
        user: TestUser
    ): Promise<string> {
        const login = await request.post(`${baseURL}/api/public/login`, {
            data: { email: user.email, password: user.password }
        });
        const { token } = (await login.json()) as { token: string };
        return token;
    }

    public static async createIssuesViaApi(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        idState: number,
        titles: string[]
    ): Promise<void> {
        for (const title of titles) {
            const res = await request.post(`${baseURL}/api/private/project/${idProject}/issue`, {
                headers: { Authorization: token },
                data: { title, idState, description: 'Created by an e2e test.' }
            });
            expect(res.status(), await res.text()).toBe(200);
        }
    }

    public static async setCustomFieldValues(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        idIssue: number,
        customFields: Record<string, string | number | boolean | null>
    ): Promise<void> {
        const res = await request.patch(
            `${baseURL}/api/private/project/${idProject}/issue/${idIssue}`,
            { headers: { Authorization: token }, data: { customFields } }
        );
        expect(res.status(), await res.text()).toBe(200);
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
        // Waits on the new row, not on the revealed panel: an earlier key leaves that
        // panel open, so it would already be visible before this key is created.
        const rows = page.getByTestId('user-api-key-row');
        const rowsBefore = await rows.count();

        await page.getByTestId('user-api-key-name').fill(name);
        await page.getByTestId('user-api-key-create').click();
        await expect(rows).toHaveCount(rowsBefore + 1);

        const revealed = page.getByTestId('user-api-key-revealed');
        await expect(revealed).toBeVisible();
        return (await revealed.locator('code').innerText()).trim();
    }

    public static async fillWikiEditor(page: Page, text: string): Promise<void> {
        const content = page.getByTestId('wiki-editor').locator('.cm-content');
        await content.click();
        await page.keyboard.press('ControlOrMeta+a');
        await page.keyboard.press('Delete');
        await page.keyboard.insertText(text);
    }

    public static async createWikiPage(
        page: Page,
        idProject: number,
        title: string,
        body: string
    ): Promise<void> {
        await page.goto(`/project/${idProject}/wiki/new?space=project`);
        await page.getByTestId('wiki-edit-title').fill(title);
        await Interaction.fillWikiEditor(page, body);
        await page.getByTestId('wiki-save').click();
        await expect(page.getByTestId('wiki-page-title')).toHaveText(title);
    }

    public static async acceptConfirm(page: Page): Promise<void> {
        await page.locator('.ui-confirm-panel').getByRole('button', { name: 'Yes' }).click();
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

    // grabOffsetY moves the grab point away from the handle's vertical centre, for
    // handles whose centre holds a button that would take the pointerdown instead.
    public static async dragBy(
        page: Page,
        handle: Locator,
        deltaX: number,
        grabOffsetY?: number
    ): Promise<void> {
        await Interaction.waitForStableBox(handle);
        const box = (await handle.boundingBox())!;
        const startX = box.x + box.width / 2;
        const y = box.y + (grabOffsetY ?? box.height / 2);

        await page.mouse.move(startX, y);
        await page.mouse.down();
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
