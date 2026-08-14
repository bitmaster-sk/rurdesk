import { test, expect, APIRequestContext, Page } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';
import { assignToSprint, createSprint, isoDay, tokenOf } from './support/sprint';

const STRIP = '[data-testid="sprint-health-strip"]';
const PROGRESS = '[data-testid="sprint-health-progress"]';
const VERDICT = '[data-testid="sprint-health-verdict"]';
const UNIT = '[data-testid="sprint-health-unit"]';

async function setPoints(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    idProject: number,
    idIssuePublic: number,
    points: number
): Promise<void> {
    const url = `${baseURL}/api/private/project/${idProject}/issue/${idIssuePublic}`;
    const current = await request.get(url, { headers: { Authorization: token } });
    expect(current.status(), 'reading the issue').toBe(200);
    const issue = (await current.json()) as {
        idState: number;
        idSeverity: number;
        title: string;
        description: string;
        estimated: number;
    };

    const res = await request.patch(url, {
        headers: { Authorization: token },
        data: {
            idState: issue.idState,
            idSeverity: issue.idSeverity,
            title: issue.title,
            description: issue.description,
            estimated: issue.estimated,
            points
        }
    });
    expect(res.status(), 'setting points').toBe(200);
}

async function openSprintTab(page: Page, idProject: number, tab: string | RegExp): Promise<void> {
    await page.goto(`/project/${idProject}/issue/view/kanban`);
    await expect(page.locator(STRIP)).toBeVisible();
    await page.getByRole('button', { name: tab }).first().click();
}

test('the strip counts the backlog and follows the selected tab', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'sprint-strip-backlog');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Strip Backlog Project');
    const token = await tokenOf(request, baseURL!, user);

    const idIssuePublic = await Interaction.createIssue(page, idProject, {
        title: 'Backlog task one',
        description: 'x',
        state: 'New',
        severity: 'Low'
    });

    await page.goto(`/project/${idProject}/issue/view/kanban`);
    await expect(page.locator(PROGRESS)).toContainText('1 task ');
    await expect(page.locator(PROGRESS)).toContainText('open, in no sprint');

    const idSprint = await createSprint(request, baseURL!, token, idProject, {
        name: 'Cycle A',
        startAt: isoDay(0),
        endAt: isoDay(14)
    });
    await assignToSprint(request, baseURL!, token, idProject, idIssuePublic, idSprint);

    await openSprintTab(page, idProject, 'Cycle A');
    await expect(page.locator(PROGRESS)).toContainText('0/1 task');

    await page.getByRole('button', { name: 'Backlog', exact: true }).click();
    await expect(page.locator(PROGRESS)).toContainText('0 tasks');
});

test('the strip reflects progress and verdict of a running cycle', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'sprint-strip-running');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Strip Running Project');
    const token = await tokenOf(request, baseURL!, user);

    const idSprint = await createSprint(request, baseURL!, token, idProject, {
        name: 'Cycle B',
        startAt: isoDay(-9),
        endAt: isoDay(5)
    });

    const done = await Interaction.createIssue(page, idProject, {
        title: 'Finished task',
        description: 'x',
        state: 'Closed',
        severity: 'Low'
    });
    const open = await Interaction.createIssue(page, idProject, {
        title: 'Unfinished task',
        description: 'x',
        state: 'New',
        severity: 'Low'
    });
    await assignToSprint(request, baseURL!, token, idProject, done, idSprint);
    await assignToSprint(request, baseURL!, token, idProject, open, idSprint);

    await openSprintTab(page, idProject, 'Cycle B');

    await expect(page.locator(PROGRESS)).toContainText('1/2 tasks');
    await expect(page.locator(STRIP)).toContainText('days left');
    await expect(page.locator(STRIP)).toContainText('no points set');
    await expect(page.locator(UNIT)).toContainText('Tasks');
    await expect(page.locator(VERDICT)).toContainText('behind');
});

test('a cycle that has not started yet reports that, with no forecast', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'sprint-strip-planned');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Strip Planned Project');
    const token = await tokenOf(request, baseURL!, user);

    const idSprint = await createSprint(request, baseURL!, token, idProject, {
        name: 'Cycle C',
        startAt: isoDay(3),
        endAt: isoDay(17)
    });
    const idIssuePublic = await Interaction.createIssue(page, idProject, {
        title: 'Planned task',
        description: 'x',
        state: 'New',
        severity: 'Low'
    });
    await assignToSprint(request, baseURL!, token, idProject, idIssuePublic, idSprint);

    await openSprintTab(page, idProject, 'Cycle C');

    await expect(page.locator(PROGRESS)).toContainText('1 task planned');
    await expect(page.locator(STRIP)).toContainText('starts in 3 days');
    await expect(page.locator(STRIP)).not.toContainText('too early to forecast');
    await expect(page.locator(STRIP)).not.toContainText('days left');
});

test('the Points/Tasks toggle switches every number, and locks when nothing is pointed', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'sprint-strip-unit');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Strip Unit Project');
    const token = await tokenOf(request, baseURL!, user);

    const idSprint = await createSprint(request, baseURL!, token, idProject, {
        name: 'Cycle D',
        startAt: isoDay(-9),
        endAt: isoDay(5)
    });

    const done = await Interaction.createIssue(page, idProject, {
        title: 'Pointed done',
        description: 'x',
        state: 'Closed',
        severity: 'Low'
    });
    const open = await Interaction.createIssue(page, idProject, {
        title: 'Pointed open',
        description: 'x',
        state: 'New',
        severity: 'Low'
    });
    await assignToSprint(request, baseURL!, token, idProject, done, idSprint);
    await assignToSprint(request, baseURL!, token, idProject, open, idSprint);

    await openSprintTab(page, idProject, 'Cycle D');
    await expect(page.locator(PROGRESS)).toContainText('1/2 tasks');
    await expect(page.locator(STRIP)).toContainText('no points set');
    await expect(page.locator(UNIT).locator('button', { hasText: 'Points' })).toBeDisabled();

    await setPoints(request, baseURL!, token, idProject, done, 3);
    await setPoints(request, baseURL!, token, idProject, open, 5);

    await page.reload();
    await expect(page.locator(PROGRESS)).toContainText('3/8 pts');
    await expect(page.locator(STRIP)).not.toContainText('no points set');

    await page.locator(UNIT).locator('button', { hasText: 'Tasks' }).click();
    await expect(page.locator(PROGRESS)).toContainText('1/2 tasks');

    await page.locator(UNIT).locator('button', { hasText: 'Points' }).click();
    await expect(page.locator(PROGRESS)).toContainText('3/8 pts');
});
