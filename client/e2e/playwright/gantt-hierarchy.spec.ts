import { APIRequestContext, expect, test } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';
import { authHeaders } from './support/auth';
import { stateIdOf } from './support/sprint';

interface CreatedIssue {
    idIssuePublic: number;
    scheduledAt: string | null;
}

abstract class Hierarchy {
    public static async createIssue(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        data: Record<string, unknown>
    ): Promise<CreatedIssue> {
        const res = await request.post(`${baseURL}/api/private/project/${idProject}/issue`, {
            headers: authHeaders(token),
            data: { description: 'Created by an e2e test.', ...data }
        });
        expect(res.status(), await res.text()).toBe(200);
        return res.json();
    }

    public static async link(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        idParent: number,
        idChild: number
    ): Promise<void> {
        const res = await request.post(
            `${baseURL}/api/private/project/${idProject}/issue/${idParent}/relation`,
            {
                headers: authHeaders(token),
                data: { idIssuePublicTo: idChild, relationType: 'hierarchy' }
            }
        );
        expect(res.status(), await res.text()).toBe(200);
    }

    public static async start(
        request: APIRequestContext,
        baseURL: string,
        token: string,
        idProject: number,
        idIssuePublic: number
    ): Promise<number> {
        const res = await request.get(
            `${baseURL}/api/private/project/${idProject}/issue/${idIssuePublic}`,
            { headers: authHeaders(token) }
        );
        const issue = (await res.json()) as CreatedIssue;
        return new Date(issue.scheduledAt!).getTime();
    }
}

// Scenario:
// - create a dedicated non-admin user, log in and create a blank project
// - create a parent task without dates and two scheduled children, linked as Parent / Child
// - open the gantt and assert the parent is a summary row numbered 1 with children 1.1 and 1.2
// - assert the parent bar is drawn as a summary bar without a resize handle
// - drag the summary bar to the right
// - assert both children moved by the same, non-zero amount
test('dragging a summary bar moves all of its subtasks', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'gantt-hierarchy');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Gantt Hierarchy Project');
    const token = await Interaction.apiToken(request, baseURL!, user);
    const idState = await stateIdOf(request, baseURL!, token, idProject, 'New');

    const today = new Date();
    today.setUTCHours(8, 0, 0, 0);
    const tomorrow = new Date(today.getTime() + 24 * 3600 * 1000);
    const parent = await Hierarchy.createIssue(request, baseURL!, token, idProject, {
        title: 'Build the website',
        idState
    });
    const first = await Hierarchy.createIssue(request, baseURL!, token, idProject, {
        title: 'Design the layout',
        idState,
        scheduledAt: today.toISOString(),
        estimated: 8 * 3600
    });
    const second = await Hierarchy.createIssue(request, baseURL!, token, idProject, {
        title: 'Write the pages',
        idState,
        scheduledAt: tomorrow.toISOString(),
        estimated: 8 * 3600
    });
    for (const child of [first, second]) {
        await Hierarchy.link(
            request,
            baseURL!,
            token,
            idProject,
            parent.idIssuePublic,
            child.idIssuePublic
        );
    }

    await page.goto(`/project/${idProject}/issue/view/gantt`);
    const row = (id: number) => page.locator(`.wbs-row[data-task-id="${id}"]`);
    await expect(row(parent.idIssuePublic).locator('.wbs-row--code')).toHaveText('1');
    await expect(row(first.idIssuePublic).locator('.wbs-row--code')).toHaveText('1.1');
    await expect(row(second.idIssuePublic).locator('.wbs-row--code')).toHaveText('1.2');

    const summaryBar = page.locator(`.gantt-bar[data-task-id="${parent.idIssuePublic}"]`);
    await expect(summaryBar).toHaveClass(/gantt-bar--summary/);
    await expect(summaryBar.locator('.gantt-bar--resize-handle')).toHaveCount(0);

    const before = await Promise.all(
        [first, second].map(c =>
            Hierarchy.start(request, baseURL!, token, idProject, c.idIssuePublic)
        )
    );
    const barBox = (await summaryBar.boundingBox())!;
    await Interaction.dragBy(page, summaryBar, Math.max(120, barBox.width));

    await expect
        .poll(
            async () =>
                (await Hierarchy.start(request, baseURL!, token, idProject, first.idIssuePublic)) -
                before[0]
        )
        .toBeGreaterThan(0);
    const after = await Promise.all(
        [first, second].map(c =>
            Hierarchy.start(request, baseURL!, token, idProject, c.idIssuePublic)
        )
    );
    expect(after[0] - before[0]).toBeGreaterThan(0);
    expect(after[1] - before[1]).toBe(after[0] - before[0]);
});
