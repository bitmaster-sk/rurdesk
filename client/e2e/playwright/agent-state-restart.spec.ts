import { expect, test } from '@playwright/test';
import { createUser } from './support/user';
import { tokenOf } from './support/sprint';
import { assignAgent, createStubGatewayBot } from './support/agent-bot';
import { Interaction } from './support/interaction';
import { WorkflowEventMap } from './support/workflow-event-map';

const RUN_TIMEOUT_MS = 90_000;

test.describe('workflow event state on restart', () => {
    test.slow();

    // Scenario:
    // - map queued, in_progress, awaiting_approval and cancelled to four states
    // - assign an agent and let the run park at the design review
    // - restart the run
    // - assert the restart does not leave the task on the cancelled state: the
    //   old run is cancelled and the new one is queued in the same request, so
    //   queued must win
    // - assert the new run walks the state machine again, from in_progress to
    //   awaiting_approval
    test('a restart runs the state machine again from the start', async ({
        page,
        request,
        baseURL
    }) => {
        const user = await createUser(request, baseURL!, 'restart');
        await Interaction.login(page, user);
        const idProject = await Interaction.createBlankProject(page, 'Restart Flow');

        const adminToken = await tokenOf(request, baseURL!, {
            name: 'E2E Admin',
            email: 'e2e-admin@example.com',
            password: 'Passw0rd!23'
        });
        const userToken = await tokenOf(request, baseURL!, user);

        const idStateQueued = await WorkflowEventMap.createState(
            request,
            baseURL!,
            userToken,
            idProject,
            'Restart queued'
        );
        const idStateInProgress = await WorkflowEventMap.createState(
            request,
            baseURL!,
            userToken,
            idProject,
            'Restart in progress'
        );
        const idStateApproval = await WorkflowEventMap.createState(
            request,
            baseURL!,
            userToken,
            idProject,
            'Restart approval'
        );
        const idStateCancelled = await WorkflowEventMap.createState(
            request,
            baseURL!,
            userToken,
            idProject,
            'Restart cancelled'
        );
        await WorkflowEventMap.mapEvents(request, baseURL!, userToken, idProject, [
            { event: 'queued', idState: idStateQueued },
            { event: 'in_progress', idState: idStateInProgress },
            { event: 'awaiting_approval', idState: idStateApproval },
            { event: 'cancelled', idState: idStateCancelled }
        ]);

        const idIssuePublic = await Interaction.createIssue(page, idProject, {
            title: 'Restart task',
            description: 'Restarting the run must replay the state machine.',
            state: 'New',
            severity: 'Medium'
        });

        const bot = await createStubGatewayBot(request, baseURL!, adminToken, idProject, 'restart');
        const idRun = await assignAgent(
            request,
            baseURL!,
            userToken,
            idProject,
            idIssuePublic,
            bot.idUser
        );

        await WorkflowEventMap.waitForRunPhase(
            request,
            baseURL!,
            userToken,
            idRun,
            'awaiting_approval',
            RUN_TIMEOUT_MS
        );
        expect(
            await WorkflowEventMap.issueState(
                request,
                baseURL!,
                userToken,
                idProject,
                idIssuePublic
            ),
            'the first run parks on the awaiting_approval state'
        ).toBe(idStateApproval);

        const idNewRun = await WorkflowEventMap.restartRun(request, baseURL!, userToken, idRun);
        expect(idNewRun, 'restart creates a new run').not.toBe(idRun);
        expect(
            await WorkflowEventMap.runPhase(request, baseURL!, userToken, idRun),
            'the old run is cancelled'
        ).toBe('cancelled');

        // Both mappings fire in one request, and queued is written second.
        expect(
            await WorkflowEventMap.issueState(
                request,
                baseURL!,
                userToken,
                idProject,
                idIssuePublic
            ),
            'a restart must not strand the task on the cancelled state'
        ).not.toBe(idStateCancelled);

        await WorkflowEventMap.waitForRunPhase(
            request,
            baseURL!,
            userToken,
            idNewRun,
            'in_progress',
            RUN_TIMEOUT_MS
        );
        expect(
            await WorkflowEventMap.issueState(
                request,
                baseURL!,
                userToken,
                idProject,
                idIssuePublic
            ),
            'the new run drives the mapping again'
        ).toBe(idStateInProgress);

        await WorkflowEventMap.waitForRunPhase(
            request,
            baseURL!,
            userToken,
            idNewRun,
            'awaiting_approval',
            RUN_TIMEOUT_MS
        );
        expect(
            await WorkflowEventMap.issueState(
                request,
                baseURL!,
                userToken,
                idProject,
                idIssuePublic
            ),
            'the new run parks on the awaiting_approval state again'
        ).toBe(idStateApproval);
    });
});
