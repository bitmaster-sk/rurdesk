import { expect, test } from '@playwright/test';
import { assignAgent, createStubGatewayAgent } from './support/agent';
import { Attempt } from './support/attempt';
import { Interaction } from './support/interaction';
import { tokenOf } from './support/sprint';
import { createUser } from './support/user';
import { WorkflowEventMap } from './support/workflow-event-map';

const RUN_TIMEOUT_MS = 90_000;

test.describe('agent proposes wiki changes', () => {
    test.slow();

    // Scenario:
    // - create a user, log in, create a blank project with a git integration and an issue
    // - create the wiki page "Deploy runbook" through the API (v1)
    // - create an agent whose implementation stage proposes a change of that page
    // - assign the agent, approve the design and the plan, and wait for the open pull request
    // - assert the task shows the wiki proposal card as open
    // - open the proposal, approve it and assert it waits for the merge without counting in the wiki
    // - merge the pull request and wait for the run to finish
    // - assert the proposal is published and the page shows the proposed text as v2 noted with the task and run
    test('a proposal approved before the merge is published when the pull request merges', async ({
        page,
        request,
        baseURL
    }) => {
        const label = Attempt.label('wiki-proposal');
        const user = await createUser(request, baseURL!, label);
        await Interaction.login(page, user);
        const idProject = await Interaction.createBlankProject(page, `Wiki Proposal ${label}`);
        const token = await Interaction.apiToken(request, baseURL!, user);
        await WorkflowEventMap.createGitIntegration(request, baseURL!, token, idProject, label);
        const idIssuePublic = await Interaction.createIssue(page, idProject, {
            title: 'Move the deploy script',
            description: 'The deploy script now lives in scripts/.',
            state: 'New',
            severity: 'Medium'
        });

        const created = await request.post(
            `${baseURL}/api/private/project/${idProject}/wiki/page`,
            {
                headers: { Authorization: token },
                data: {
                    space: 'project',
                    title: 'Deploy runbook',
                    summary: 'How to deploy',
                    body: 'Run ./deploy.sh from the repository root.'
                }
            }
        );
        expect(created.status(), await created.text()).toBe(200);

        const adminToken = await tokenOf(request, baseURL!, {
            name: 'E2E Admin',
            email: 'e2e-admin@example.com',
            password: 'Passw0rd!23'
        });
        const agent = await createStubGatewayAgent(
            request,
            baseURL!,
            adminToken,
            idProject,
            label,
            {
                implementation: {
                    wikiProposals: [
                        {
                            kind: 'update',
                            slug: 'deploy-runbook',
                            baseVersion: 1,
                            body: 'Run scripts/deploy.sh from the repository root.',
                            reason: 'The deploy script moved to scripts/.'
                        }
                    ]
                }
            }
        );
        const idRun = await assignAgent(
            request,
            baseURL!,
            token,
            idProject,
            idIssuePublic,
            agent.idUser
        );
        for (let approval = 0; approval < 2; approval++) {
            await WorkflowEventMap.waitForRunPhase(
                request,
                baseURL!,
                token,
                idRun,
                'awaiting_approval',
                RUN_TIMEOUT_MS
            );
            await WorkflowEventMap.approveRun(request, baseURL!, token, idRun);
        }
        await WorkflowEventMap.waitForRunPhase(
            request,
            baseURL!,
            token,
            idRun,
            'pr_open',
            RUN_TIMEOUT_MS
        );

        await page.goto(`/project/${idProject}/issue/${idIssuePublic}`);
        const card = page.getByTestId('wiki-proposal-card');
        await expect(card).toContainText('The deploy script moved to scripts/.');
        await expect(card.getByTestId('wiki-proposal-card-state')).toHaveText('open');

        await card.getByTestId('wiki-proposal-card-open').click();
        await expect(page).toHaveURL(new RegExp(`/project/${idProject}/wiki/proposals/\\d+$`));
        await expect(page.getByTestId('wiki-proposal-title')).toHaveText('Deploy runbook');
        await expect(page.getByTestId('wiki-proposal-open')).toBeVisible();

        await page.getByTestId('wiki-proposal-accept').click();
        await expect(page.getByTestId('wiki-proposal-state')).toHaveText(
            'approved · publishes on merge'
        );
        await expect(page.getByTestId('wiki-proposal-accept')).toHaveCount(0);
        await expect(page.getByTestId('wiki-proposals-count')).toHaveCount(0);

        await WorkflowEventMap.setPrState(WorkflowEventMap.repoPathFor(label), 'closed', true);
        await WorkflowEventMap.waitForRunPhase(
            request,
            baseURL!,
            token,
            idRun,
            'done',
            RUN_TIMEOUT_MS
        );

        await expect(page.getByTestId('wiki-proposal-state')).toHaveText('published', {
            timeout: RUN_TIMEOUT_MS
        });
        await expect(page.getByTestId('wiki-proposals-count')).toHaveCount(0);

        await page.getByTestId('wiki-proposal-page').click();
        await expect(page.getByTestId('wiki-page-body')).toContainText(
            'Run scripts/deploy.sh from the repository root.'
        );
        await page.getByTestId('wiki-history-link').click();
        await expect(page.getByTestId('wiki-version')).toHaveCount(2);
        await expect(page.getByTestId('wiki-version').first()).toContainText(
            `Agent proposal from #${idIssuePublic} (run #${idRun})`
        );
    });
});
