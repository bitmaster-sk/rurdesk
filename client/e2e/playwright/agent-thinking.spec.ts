import { test, expect } from '@playwright/test';
import { createUser } from './support/user';
import { tokenOf } from './support/sprint';
import { assignAgent, createStubGatewayBot } from './support/agent-bot';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Thinking Project';
const ISSUE_TITLE = 'Watch the agent think';

// The stub gateway streams three batches per stage, ~700ms apart, and the run
// walks pickup → brainstorming → design before it parks for approval.
const RUN_TIMEOUT_MS = 60_000;

test.describe('agent thinking', () => {
    // The stub gateway holds one bot's credentials at a time, so two tests
    // configuring it at once would authenticate each other's callbacks away.
    test.describe.configure({ mode: 'serial' });
    test.slow();

    // Scenario:
    // - create a user and log in through the real login form
    // - create a blank project and an issue in it
    // - create a bot pointed at the stub gateway and hand the stub its tokens
    // - assign the agent while the issue detail is open, without reloading
    // - assert a thinking row appears with the working dot
    // - assert the streamed thought and the tool call arrive in the open page
    test('streams the agent thinking into the open issue', async ({ page, request, baseURL }) => {
        const user = await createUser(request, baseURL!, 'agent-thinking');
        await Interaction.login(page, user);

        const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);
        const idIssuePublic = await Interaction.createIssue(page, idProject, {
            title: ISSUE_TITLE,
            description: 'The agent thinks out loud here.',
            state: 'New',
            severity: 'Medium'
        });

        const adminToken = await tokenOf(request, baseURL!, {
            name: 'E2E Admin',
            email: 'e2e-admin@example.com',
            password: 'Passw0rd!23'
        });
        const bot = await createStubGatewayBot(
            request,
            baseURL!,
            adminToken,
            idProject,
            'thinking'
        );

        const userToken = await tokenOf(request, baseURL!, user);
        await assignAgent(request, baseURL!, userToken, idProject, idIssuePublic, bot.idUser);

        const row = page.getByTestId('agent-thinking-row').first();
        await expect(row).toBeVisible({ timeout: RUN_TIMEOUT_MS });
        await expect(page.getByTestId('agent-thinking-working').first()).toBeVisible();

        const body = page.getByTestId('agent-thinking-body').first();
        await expect(body).toContainText('Picking up the', { timeout: RUN_TIMEOUT_MS });
        await expect(body).toContainText('developer__shell', { timeout: RUN_TIMEOUT_MS });
    });

    // Scenario:
    // - create a user, a project and an issue, and a bot on the stub gateway
    // - assign the agent and wait until the design stage posts its comment
    // - reload, so nothing is left of the live stream in memory
    // - expand the design stage's thinking row
    // - assert the thinking the agent recorded is replayed from the server
    test('replays a finished stage thinking after a reload', async ({ page, request, baseURL }) => {
        const user = await createUser(request, baseURL!, 'agent-thinking-replay');
        await Interaction.login(page, user);

        const idProject = await Interaction.createBlankProject(page, `${PROJECT_NAME} Replay`);
        const idIssuePublic = await Interaction.createIssue(page, idProject, {
            title: ISSUE_TITLE,
            description: 'The agent thinks out loud here.',
            state: 'New',
            severity: 'Medium'
        });

        const adminToken = await tokenOf(request, baseURL!, {
            name: 'E2E Admin',
            email: 'e2e-admin@example.com',
            password: 'Passw0rd!23'
        });
        const bot = await createStubGatewayBot(request, baseURL!, adminToken, idProject, 'replay');

        const userToken = await tokenOf(request, baseURL!, user);
        await assignAgent(request, baseURL!, userToken, idProject, idIssuePublic, bot.idUser);

        await expect(page.getByText('Stub design proposal.')).toBeVisible({
            timeout: RUN_TIMEOUT_MS
        });

        await page.reload();

        const designRow = page
            .getByTestId('agent-thinking-row')
            .filter({ hasText: 'Design' })
            .first();
        await expect(designRow).toBeVisible({ timeout: RUN_TIMEOUT_MS });
        await designRow.getByTestId('agent-thinking-toggle').click();

        await expect(designRow.getByTestId('agent-thinking-body')).toContainText(
            'Picking up the design stage'
        );
    });
});
