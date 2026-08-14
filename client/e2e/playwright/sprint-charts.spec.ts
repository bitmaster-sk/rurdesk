import { test, expect, Page } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';
import {
    assignToSprint,
    createSprint,
    editIssue,
    isoDay,
    stateIdOf,
    tokenOf
} from './support/sprint';

const BAND = '[data-testid="sprint-charts-band"]';
const SCOPE = '[data-testid="sprint-charts-scope"]';
const FORECAST = '[data-testid="sprint-charts-forecast"]';
const AVG = '[data-testid="sprint-charts-avg"]';
const CHARTS_TOGGLE = '[data-testid="kanban-charts-toggle"] button';
const BURNDOWN = '[data-testid="sprint-burndown-chart"]';
const VELOCITY_EMPTY = '[data-testid="sprint-velocity-empty"]';
const PROGRESS = '[data-testid="sprint-health-progress"]';

async function openCharts(page: Page, idProject: number, tab: string): Promise<void> {
    await page.goto(`/project/${idProject}/issue/view/kanban`);
    await page.getByRole('button', { name: tab }).first().click();
    if ((await page.locator(BAND).count()) === 0) {
        await page.locator(CHARTS_TOGGLE).click();
    }
    await expect(page.locator(BAND)).toBeVisible();
}

// - create a project with a cycle that started nine days ago and runs five more
// - put one finished 3-pt task and one open 5-pt task in it
// - open the board, switch to that cycle and turn the Charts band on
// - check all three left-column figures render: scope unchanged (one recorded
//   day), a forecast past the planned end, and no average without closed cycles
test('the charts band fills its three figures for a running cycle', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'sprint-charts-figures');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Charts Figures Project');
    const token = await tokenOf(request, baseURL!, user);

    const idSprint = await createSprint(request, baseURL!, token, idProject, {
        name: 'Cycle Charts',
        startAt: isoDay(-9),
        endAt: isoDay(5)
    });

    const done = await Interaction.createIssue(page, idProject, {
        title: 'Charts finished task',
        description: 'x',
        state: 'Closed',
        severity: 'Low'
    });
    const open = await Interaction.createIssue(page, idProject, {
        title: 'Charts unfinished task',
        description: 'x',
        state: 'New',
        severity: 'Low'
    });
    await assignToSprint(request, baseURL!, token, idProject, done, idSprint);
    await assignToSprint(request, baseURL!, token, idProject, open, idSprint);
    await editIssue(request, baseURL!, token, idProject, done, { points: 3 });
    await editIssue(request, baseURL!, token, idProject, open, { points: 5 });

    await openCharts(page, idProject, 'Cycle Charts');

    await expect(page.locator(BAND)).toContainText('Cycle Charts');
    await expect(page.locator(BURNDOWN)).toBeVisible();

    await expect(page.locator(SCOPE)).toContainText('unchanged');
    await expect(page.locator(FORECAST)).not.toHaveText('—');
    await expect(page.locator(BAND)).toContainText('past the sprint end');
    await expect(page.locator(AVG)).toHaveText('—');
    await expect(page.locator(VELOCITY_EMPTY)).toBeVisible();
});

// - create a project with a running cycle holding one open task
// - open the board on that cycle with the Charts band on
// - close the task through the API, as a teammate would
// - check the board re-reads the burndown and the strip counts the task as done,
//   with no page reload in between
test('the charts follow a task being finished, without a reload', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'sprint-charts-live');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Charts Live Project');
    const token = await tokenOf(request, baseURL!, user);

    const idSprint = await createSprint(request, baseURL!, token, idProject, {
        name: 'Cycle Live',
        startAt: isoDay(-9),
        endAt: isoDay(5)
    });
    const idIssuePublic = await Interaction.createIssue(page, idProject, {
        title: 'Live task',
        description: 'x',
        state: 'New',
        severity: 'Low'
    });
    await assignToSprint(request, baseURL!, token, idProject, idIssuePublic, idSprint);

    await openCharts(page, idProject, 'Cycle Live');
    await expect(page.locator(PROGRESS)).toContainText('0/1 task');

    const idClosed = await stateIdOf(request, baseURL!, token, idProject, 'Closed');
    const refetched = page.waitForRequest(candidate => candidate.url().includes('/burndown'), {
        timeout: 15_000
    });
    await editIssue(request, baseURL!, token, idProject, idIssuePublic, { idState: idClosed });

    await refetched;
    await expect(page.locator(PROGRESS)).toContainText('1/1 task');
    await expect(page.locator(BURNDOWN)).toBeVisible();
});

// - open the board of a project that has no cycles and check Charts is disabled
// - create a cycle, reload, and turn Charts on
// - reload again and check the band comes back by itself
test('the Charts toggle is remembered and is dead for a project with no cycles', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'sprint-charts-toggle');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Charts Toggle Project');
    const token = await tokenOf(request, baseURL!, user);

    await page.goto(`/project/${idProject}/issue/view/kanban`);
    await expect(page.locator(CHARTS_TOGGLE)).toBeDisabled();

    await createSprint(request, baseURL!, token, idProject, {
        name: 'Cycle Toggle',
        startAt: isoDay(-1),
        endAt: isoDay(13)
    });

    await page.reload();
    await page.locator(CHARTS_TOGGLE).click();
    await expect(page.locator(BAND)).toBeVisible();

    await page.reload();
    await expect(page.locator(BAND)).toBeVisible();
});
