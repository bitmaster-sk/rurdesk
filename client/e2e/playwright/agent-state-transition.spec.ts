import { APIRequestContext, expect, Page, test } from '@playwright/test';
import { createUser } from './support/user';
import { tokenOf } from './support/sprint';
import { assignAgent, createStubGatewayBot, GatewayScript } from './support/agent-bot';
import { Interaction } from './support/interaction';
import { WorkflowEventMap } from './support/workflow-event-map';
import { IssueNoticeLog } from './support/issue-notices';

// The stub gateway streams three thinking batches per stage ~700ms apart.
const RUN_TIMEOUT_MS = 90_000;

interface TransitionFixture {
    idProject: number;
    idIssuePublic: number;
    idRun: number;
    idState: number;
    userToken: string;
    stateName: string;
    notices: IssueNoticeLog;
}

/** Maps exactly one event, so no later event can satisfy the assertion. */
async function setUpTransition(
    page: Page,
    request: APIRequestContext,
    baseURL: string,
    label: string,
    event: string,
    script?: GatewayScript,
    withGitIntegration = false
): Promise<TransitionFixture> {
    // Attach before the first navigation, so no notice is missed.
    const notices = IssueNoticeLog.attach(page);

    const user = await createUser(request, baseURL, label);
    await Interaction.login(page, user);

    const idProject = await Interaction.createBlankProject(page, `Transition ${label}`);

    const adminToken = await tokenOf(request, baseURL, {
        name: 'E2E Admin',
        email: 'e2e-admin@example.com',
        password: 'Passw0rd!23'
    });
    const userToken = await tokenOf(request, baseURL, user);

    // The state must exist before the issue page loads, or the dropdown renders it blank.
    const stateName = `Moved ${label}`;
    const idState = await WorkflowEventMap.createState(
        request,
        baseURL,
        userToken,
        idProject,
        stateName
    );
    await WorkflowEventMap.mapEventToState(request, baseURL, userToken, idProject, event, idState);

    if (withGitIntegration) {
        await WorkflowEventMap.createGitIntegration(request, baseURL, userToken, idProject, label);
    }

    const idIssuePublic = await Interaction.createIssue(page, idProject, {
        title: `Transition ${label} task`,
        description: 'The workflow event mapping drives this task state.',
        state: 'New',
        severity: 'Medium'
    });

    const bot = await createStubGatewayBot(request, baseURL, adminToken, idProject, label, script);
    const idRun = await assignAgent(
        request,
        baseURL,
        userToken,
        idProject,
        idIssuePublic,
        bot.idUser
    );

    return { idProject, idIssuePublic, idRun, idState, userToken, stateName, notices };
}

/** Asserts the notice carried the state, the open page shows it, and a reload keeps it. */
async function expectStateLiveAndPersisted(page: Page, fixture: TransitionFixture): Promise<void> {
    await fixture.notices.waitForState(fixture.idState, RUN_TIMEOUT_MS);
    await expect(page.getByTestId('issue-state')).toContainText(fixture.stateName, {
        timeout: RUN_TIMEOUT_MS
    });

    await page.reload();
    await expect(page.getByTestId('issue-state')).toContainText(fixture.stateName, {
        timeout: RUN_TIMEOUT_MS
    });
}

test.describe('workflow event state transitions', () => {
    test.slow();

    // Scenario:
    // - create a user, a project and a task, and map the `queued` event to a state
    // - assign an agent, which creates the run in the queued phase
    // - assert an issue notice carried the mapped state and the open page shows it
    // - reload and assert the mapped state again
    test('queued applies its mapping when the run is created', async ({
        page,
        request,
        baseURL
    }) => {
        const fixture = await setUpTransition(page, request, baseURL!, 'queued', 'queued');
        await expectStateLiveAndPersisted(page, fixture);
    });

    // Scenario:
    // - map the `in_progress` event to a state
    // - assign an agent and let the scheduler dispatch the pickup stage
    // - assert an issue notice carried the mapped state and the open page shows it
    // - reload and assert the mapped state again
    test('in_progress applies its mapping when the scheduler dispatches', async ({
        page,
        request,
        baseURL
    }) => {
        const fixture = await setUpTransition(page, request, baseURL!, 'inprogress', 'in_progress');
        await expectStateLiveAndPersisted(page, fixture);
    });

    // Scenario:
    // - map the `awaiting_input` event to a state
    // - script the stub gateway to ask a question in the brainstorming stage
    // - assert an issue notice carried the mapped state and the open page shows it
    // - reload and assert the mapped state again
    test('awaiting_input applies its mapping when the agent asks a question', async ({
        page,
        request,
        baseURL
    }) => {
        const fixture = await setUpTransition(
            page,
            request,
            baseURL!,
            'awaitinput',
            'awaiting_input',
            {
                brainstorming: {
                    outcome: 'question_asked',
                    message: 'Which database should this use?',
                    messageKind: 'brainstorming_question'
                }
            }
        );
        await expectStateLiveAndPersisted(page, fixture);
    });

    // Scenario:
    // - map the `awaiting_approval` event to a state
    // - assign an agent and let the run walk to the design stage, which parks for review
    // - assert an issue notice carried the mapped state and the open page shows it
    // - reload and assert the mapped state again
    test('awaiting_approval applies its mapping when output waits for review', async ({
        page,
        request,
        baseURL
    }) => {
        const fixture = await setUpTransition(
            page,
            request,
            baseURL!,
            'awaitappr',
            'awaiting_approval'
        );
        await expectStateLiveAndPersisted(page, fixture);
    });

    // Scenario:
    // - map the `failed` event to a state
    // - script the stub gateway to error out on the pickup stage
    // - assert an issue notice carried the mapped state and the open page shows it
    // - reload and assert the mapped state again
    test('failed applies its mapping when a stage errors', async ({ page, request, baseURL }) => {
        const fixture = await setUpTransition(page, request, baseURL!, 'failed', 'failed', {
            pickup: { outcome: 'errored', errorReason: 'stub_failure' }
        });
        await expectStateLiveAndPersisted(page, fixture);
    });

    // Scenario:
    // - map the `cancelled` event to a state
    // - script the stub gateway to stall on pickup, so the run stays non-terminal
    // - cancel the run through the API
    // - assert an issue notice carried the mapped state and the open page shows it
    // - reload and assert the mapped state again
    test('cancelled applies its mapping when the run is cancelled', async ({
        page,
        request,
        baseURL
    }) => {
        const fixture = await setUpTransition(page, request, baseURL!, 'cancelled', 'cancelled', {
            pickup: { stall: true }
        });
        await WorkflowEventMap.waitForRunPhase(
            request,
            baseURL!,
            fixture.userToken,
            fixture.idRun,
            'in_progress',
            RUN_TIMEOUT_MS
        );
        await WorkflowEventMap.cancelRun(request, baseURL!, fixture.userToken, fixture.idRun);

        await expectStateLiveAndPersisted(page, fixture);
    });

    // Scenario:
    // - map the `pr_open` event to a state and give the project a git integration
    // - assign an agent and approve the design and implementation-plan stages
    // - let the implementation stage report its pull request
    // - assert an issue notice carried the mapped state and the open page shows it
    // - reload and assert the mapped state again
    test('pr_open applies its mapping when the agent opens a pull request', async ({
        page,
        request,
        baseURL
    }) => {
        const fixture = await setUpTransition(
            page,
            request,
            baseURL!,
            'propen',
            'pr_open',
            undefined,
            true
        );

        // design and implementation_plan each park for review before the PR opens.
        for (let approval = 0; approval < 2; approval++) {
            await WorkflowEventMap.waitForRunPhase(
                request,
                baseURL!,
                fixture.userToken,
                fixture.idRun,
                'awaiting_approval',
                RUN_TIMEOUT_MS
            );
            await WorkflowEventMap.approveRun(request, baseURL!, fixture.userToken, fixture.idRun);
        }

        await expectStateLiveAndPersisted(page, fixture);
    });
});
