import { APIRequestContext, expect, test } from '@playwright/test';
import { assignAgent, createStubGatewayAgent } from './support/agent';
import { Interaction } from './support/interaction';
import { tokenOf } from './support/sprint';
import { createUser } from './support/user';

const RUN_TIMEOUT_MS = 60_000;

async function createWikiPageViaApi(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    idProject: number,
    page: { title: string; summary: string; body: string; agentAccess: string }
): Promise<void> {
    const res = await request.post(`${baseURL}/api/private/project/${idProject}/wiki/page`, {
        headers: { Authorization: token },
        data: { space: 'project', ...page }
    });
    expect(res.status(), await res.text()).toBe(200);
}

test.describe('agent reads the wiki', () => {
    test.slow();

    // Scenario:
    // - create a user, log in, create a blank project and an issue in it
    // - create an always-read wiki page and an on-demand wiki page through the API
    // - create an agent on the stub gateway and assign it to the issue
    // - wait until the design stage posts its proposal and the run waits for approval
    // - assert the Agent Run card sums up the design stage's wiki context in one line and open it
    // - assert the always page is listed with its version and the on-demand page is only in the index
    // - open the always page from the card and assert the wiki shows it
    test('the agent run shows which wiki pages went into the prompt', async ({
        page,
        request,
        baseURL
    }) => {
        const user = await createUser(request, baseURL!, 'wiki-agent');
        await Interaction.login(page, user);
        const idProject = await Interaction.createBlankProject(page, 'Wiki Agent Project');
        const idIssuePublic = await Interaction.createIssue(page, idProject, {
            title: 'Use the team conventions',
            description: 'The agent should follow how we work.',
            state: 'New',
            severity: 'Medium'
        });

        const token = await Interaction.apiToken(request, baseURL!, user);
        await createWikiPageViaApi(request, baseURL!, token, idProject, {
            title: 'How we work',
            summary: 'Team conventions',
            body: 'Every change ships with a behavior test.',
            agentAccess: 'always'
        });
        await createWikiPageViaApi(request, baseURL!, token, idProject, {
            title: 'Deploy runbook',
            summary: 'How to deploy',
            body: 'Steps to deploy.',
            agentAccess: 'on_demand'
        });

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
            'wiki'
        );
        await assignAgent(request, baseURL!, token, idProject, idIssuePublic, agent.idUser);

        await expect(page.getByText('Stub design proposal.')).toBeVisible({
            timeout: RUN_TIMEOUT_MS
        });

        const design = page.getByTestId('run-wiki-stage').filter({ hasText: 'Design' });
        await expect(design).toBeVisible({ timeout: RUN_TIMEOUT_MS });
        await expect(design.getByTestId('run-wiki-stage-toggle')).toContainText('in prompt');
        await design.getByTestId('run-wiki-stage-toggle').click();
        const always = design
            .getByTestId('run-wiki-prompt-page')
            .filter({ hasText: 'How we work' });
        await expect(always).toHaveCount(1);
        await expect(always).toContainText('How we work');
        await expect(always).toContainText('v1');
        await expect(always).toContainText('always');
        await expect(design.getByTestId('run-wiki-index')).toContainText('more pages');
        await expect(design).not.toContainText('Deploy runbook');

        await always.getByRole('link', { name: 'How we work' }).click();
        await expect(page).toHaveURL(new RegExp(`/project/${idProject}/wiki/project/how-we-work$`));
        await expect(page.getByTestId('wiki-page-title')).toHaveText('How we work');
    });
});
