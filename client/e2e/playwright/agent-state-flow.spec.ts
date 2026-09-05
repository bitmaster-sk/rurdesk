import { APIRequestContext, expect, Page, test } from '@playwright/test';
import { createUser } from './support/user';
import { tokenOf } from './support/sprint';
import { assignAgent, createStubGatewayBot } from './support/agent-bot';
import { Interaction } from './support/interaction';
import { EventMapping, WorkflowEventMap } from './support/workflow-event-map';

const RUN_TIMEOUT_MS = 90_000;

// A run visits queued → in_progress → awaiting_approval → in_progress →
// awaiting_approval → in_progress → pr_open → done; the flows below map every
// other step so each mapped one is followed by an unmapped one.

// State names are capped at 20 characters, and these tests assert on ids anyway.
const STATE_CODE: Record<string, string> = {
    queued: 'q',
    in_progress: 'ip',
    awaiting_input: 'ai',
    awaiting_approval: 'ap',
    pr_open: 'pr',
    done: 'dn',
    failed: 'fl',
    cancelled: 'cn'
};

interface FlowFixture {
    idProject: number;
    idIssuePublic: number;
    idRun: number;
    userToken: string;
    label: string;
    baseURL: string;
    request: APIRequestContext;
}

class Flow {
    private readonly fixture: FlowFixture;

    private constructor(fixture: FlowFixture) {
        this.fixture = fixture;
    }

    public static async start(
        page: Page,
        request: APIRequestContext,
        baseURL: string,
        label: string,
        buildMappings: (stateOf: (event: string) => Promise<number>) => Promise<EventMapping[]>
    ): Promise<{ flow: Flow; states: Map<string, number> }> {
        const user = await createUser(request, baseURL, label);
        await Interaction.login(page, user);
        const idProject = await Interaction.createBlankProject(page, `Flow ${label}`);

        const adminToken = await tokenOf(request, baseURL, {
            name: 'E2E Admin',
            email: 'e2e-admin@example.com',
            password: 'Passw0rd!23'
        });
        const userToken = await tokenOf(request, baseURL, user);

        const states = new Map<string, number>();
        const stateOf = async (event: string): Promise<number> => {
            const idState = await WorkflowEventMap.createState(
                request,
                baseURL,
                userToken,
                idProject,
                `${label} ${STATE_CODE[event]}`
            );
            states.set(event, idState);
            return idState;
        };
        await WorkflowEventMap.mapEvents(
            request,
            baseURL,
            userToken,
            idProject,
            await buildMappings(stateOf)
        );
        await WorkflowEventMap.createGitIntegration(request, baseURL, userToken, idProject, label);

        const idIssuePublic = await Interaction.createIssue(page, idProject, {
            title: `Flow ${label} task`,
            description: 'A full agent run walks this task through the mapped states.',
            state: 'New',
            severity: 'Medium'
        });

        const bot = await createStubGatewayBot(request, baseURL, adminToken, idProject, label);
        const idRun = await assignAgent(
            request,
            baseURL,
            userToken,
            idProject,
            idIssuePublic,
            bot.idUser
        );

        return {
            flow: new Flow({
                idProject,
                idIssuePublic,
                idRun,
                userToken,
                label,
                baseURL,
                request
            }),
            states
        };
    }

    public async state(): Promise<number | null> {
        const { request, baseURL, userToken, idProject, idIssuePublic } = this.fixture;
        return WorkflowEventMap.issueState(request, baseURL, userToken, idProject, idIssuePublic);
    }

    public async reachPhase(phase: string): Promise<void> {
        const { request, baseURL, userToken, idRun } = this.fixture;
        await WorkflowEventMap.waitForRunPhase(
            request,
            baseURL,
            userToken,
            idRun,
            phase,
            RUN_TIMEOUT_MS
        );
    }

    public async approve(): Promise<void> {
        const { request, baseURL, userToken, idRun } = this.fixture;
        await WorkflowEventMap.approveRun(request, baseURL, userToken, idRun);
    }

    public async mergePr(): Promise<void> {
        await WorkflowEventMap.setPrState(
            WorkflowEventMap.repoPathFor(this.fixture.label),
            'closed',
            true
        );
    }

    /** Waits for the phase first, because polling the state alone would race the transition. */
    public async expectStateAtPhase(phase: string, idState: number | null): Promise<void> {
        await this.reachPhase(phase);
        expect(await this.state(), `state at phase ${phase}`).toBe(idState);
    }
}

test.describe('workflow event state flows', () => {
    test.slow();

    // Scenario:
    // - map the odd steps of a run: queued, awaiting_approval, pr_open
    // - assign an agent and assert the task takes the queued state
    // - assert the unmapped in_progress step leaves that state untouched
    // - assert design parking for review takes the awaiting_approval state
    // - approve twice, asserting the unmapped in_progress steps change nothing
    // - assert the opened pull request takes the pr_open state
    // - merge the pull request and assert the unmapped done step changes nothing
    test('a run with the odd steps mapped skips the even ones without clearing', async ({
        page,
        request,
        baseURL
    }) => {
        const { flow, states } = await Flow.start(page, request, baseURL!, 'odd', async stateOf => [
            { event: 'queued', idState: await stateOf('queued') },
            { event: 'awaiting_approval', idState: await stateOf('awaiting_approval') },
            { event: 'pr_open', idState: await stateOf('pr_open') }
        ]);

        const queued = states.get('queued')!;
        const approval = states.get('awaiting_approval')!;
        const prOpen = states.get('pr_open')!;

        expect(await flow.state(), 'queued is mapped, so the run start moves the task').toBe(
            queued
        );
        await flow.expectStateAtPhase('awaiting_approval', approval);

        await flow.approve();
        await flow.expectStateAtPhase('awaiting_approval', approval);

        await flow.approve();
        await flow.expectStateAtPhase('pr_open', prOpen);

        await flow.mergePr();
        await flow.expectStateAtPhase('done', prOpen);
    });

    // Scenario:
    // - map the even steps of a run: in_progress and done
    // - assign an agent and assert the unmapped queued step leaves the task as created
    // - assert the scheduler dispatching takes the in_progress state
    // - assert the unmapped awaiting_approval step leaves that state untouched
    // - approve twice and assert the opened pull request still changes nothing
    // - merge the pull request and assert the task takes the done state
    test('a run with the even steps mapped skips the odd ones without clearing', async ({
        page,
        request,
        baseURL
    }) => {
        const { flow, states } = await Flow.start(
            page,
            request,
            baseURL!,
            'even',
            async stateOf => [
                { event: 'in_progress', idState: await stateOf('in_progress') },
                { event: 'done', idState: await stateOf('done') }
            ]
        );

        const inProgress = states.get('in_progress')!;
        const done = states.get('done')!;

        await flow.expectStateAtPhase('in_progress', inProgress);
        await flow.expectStateAtPhase('awaiting_approval', inProgress);

        await flow.approve();
        await flow.expectStateAtPhase('awaiting_approval', inProgress);

        await flow.approve();
        await flow.expectStateAtPhase('pr_open', inProgress);

        await flow.mergePr();
        await flow.expectStateAtPhase('done', done);
    });

    // Scenario:
    // - map nothing at all, and remember the state the task was created with
    // - assign an agent and let the run walk to its first review point
    // - assert the task still holds the state it was created with
    test('a run with nothing mapped leaves the task state alone', async ({
        page,
        request,
        baseURL
    }) => {
        const { flow } = await Flow.start(page, request, baseURL!, 'unmapped', async () => []);

        const created = await flow.state();
        expect(created, 'the task was created with a state').not.toBeNull();

        await flow.expectStateAtPhase('awaiting_approval', created);
    });
});
