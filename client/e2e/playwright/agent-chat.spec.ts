import { test, expect, Page } from '@playwright/test';
import { createUser } from './support/user';
import { tokenOf } from './support/sprint';
import { createStubGatewayAgent } from './support/agent';
import { authHeaders } from './support/auth';
import { Interaction } from './support/interaction';

const PROJECT_NAME = 'Chat Project';
// The stub gateway streams three thinking batches ~700ms apart before it answers.
const TURN_TIMEOUT_MS = 60_000;

async function send(page: Page, text: string): Promise<void> {
    await page.locator('.agent-chat-composer [contenteditable="true"]').fill(text);
    await page.locator('.agent-chat-composer').getByTestId('message-editor-send').click();
}

test.describe('agent chat', () => {
    // The stub gateway holds one agent's credentials at a time.
    test.describe.configure({ mode: 'serial' });
    test.slow();

    test('chats with the agent and saves its task draft as an issue', async ({
        page,
        request,
        baseURL
    }) => {
        const user = await createUser(request, baseURL!, 'agent-chat');
        await Interaction.login(page, user);
        const idProject = await Interaction.createBlankProject(page, PROJECT_NAME);

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
            'chat'
        );

        // Robot menu, then New conversation.
        await page.goto(`/project/${idProject}/issue/view/table`);
        await page.getByTestId('agent-chat-menu').click();
        await page.getByRole('menuitem', { name: 'New conversation' }).click();
        await page.waitForURL(/\/agent-chat\?.*new=1/);
        await expect(page.locator('#new-conversation-agent')).toContainText(agent.name);
        await page.getByTestId('new-conversation-start').click();
        await page.waitForURL(/\/agent-chat\/\d+/);
        const idConversation = Number(page.url().match(/\/agent-chat\/(\d+)/)![1]);

        // The thinking streams live, then the reply arrives.
        await send(page, 'Ahoj');
        const live = page.locator('[data-testid="chat-thinking"].is-live');
        await expect(live).toContainText('Picking up the chat stage.', {
            timeout: TURN_TIMEOUT_MS
        });
        await expect(live).toContainText('developer__shell', { timeout: TURN_TIMEOUT_MS });
        const firstReply = page
            .locator('.agent-chat-entry')
            .filter({ hasText: 'Stub reply: Ahoj' });
        await expect(firstReply).toBeVisible({ timeout: TURN_TIMEOUT_MS });
        await expect(live).toHaveCount(0);

        // The reply keeps what streamed behind its pill, without asking the server again.
        const thinkingRequests: string[] = [];
        page.on('request', request => {
            if (request.url().includes('/thinking')) {
                thinkingRequests.push(request.url());
            }
        });
        const pill = firstReply.getByTestId('chat-thinking-toggle');
        await expect(pill).toHaveAttribute('aria-expanded', 'false');
        await pill.click();
        await expect(pill).toHaveAttribute('aria-expanded', 'true');
        await expect(firstReply.getByTestId('chat-thinking')).toContainText(
            'Picking up the chat stage.'
        );
        expect(thinkingRequests).toEqual([]);
        await pill.click();
        await expect(firstReply.getByTestId('chat-thinking')).toHaveCount(0);

        // A reply that arrives while the owner is elsewhere counts as unread.
        await send(page, 'A ešte raz');
        await expect(page.getByTestId('chat-message-pending')).toHaveCount(0);
        await page.goto(`/project/${idProject}/issue/view/table`);
        await expect(page.getByTestId('agent-chat-badge')).toHaveText('1', {
            timeout: TURN_TIMEOUT_MS
        });

        // Opening the conversation clears the badge.
        await page.getByTestId('agent-chat-menu').click();
        await page.getByRole('menuitem', { name: 'Ahoj' }).click();
        await page.waitForURL(new RegExp(`/agent-chat/${idConversation}`));
        await expect(
            page.locator('.agent-chat-entry').filter({ hasText: 'Stub reply: A ešte raz' })
        ).toBeVisible();
        await expect(page.getByTestId('agent-chat-badge')).toHaveCount(0);

        // After leaving and reopening the conversation, the reply's thinking replays from the server.
        await firstReply.getByTestId('chat-thinking-toggle').click();
        const replay = firstReply.getByTestId('chat-thinking');
        await expect(replay).toContainText('Picking up the chat stage.');
        await expect(replay).toContainText('developer__shell');

        // Create task on the first answer, then save the draft card as an issue.
        await firstReply.hover();
        const createTask = firstReply.getByTestId('chat-create-task');
        await expect(createTask.locator('button')).toBeEnabled();
        await createTask.click();
        const card = page.getByTestId('task-draft-card');
        await expect(card).toContainText('Stub task draft: Stub reply: Ahoj', {
            timeout: TURN_TIMEOUT_MS
        });
        await card.getByTestId('task-draft-edit').click();

        await expect(page.getByTestId('task-draft-title')).toHaveValue(
            'Stub task draft: Stub reply: Ahoj'
        );
        await expect(page.getByTestId('task-draft-description')).toHaveValue(
            'Describe the work the conversation agreed on here.'
        );
        await expect(page.locator('#task-draft-project')).toContainText(PROJECT_NAME);
        await page.getByTestId('task-draft-save').click();

        await page.waitForURL(new RegExp(`/project/${idProject}/issue/[1-9]\\d*$`));
        const idIssuePublic = Number(page.url().match(/\/issue\/(\d+)$/)![1]);
        await expect(page.locator('input[formcontrolname="title"]')).toHaveValue(
            'Stub task draft: Stub reply: Ahoj'
        );

        const userToken = await tokenOf(request, baseURL!, user);
        const issue = await request.get(
            `${baseURL}/api/private/project/${idProject}/issue/${idIssuePublic}`,
            { headers: authHeaders(userToken) }
        );
        expect(issue.status()).toBe(200);
        const saved = (await issue.json()) as { description: string; assignedTo: number | null };
        expect(saved.description).toBe(
            'Describe the work the conversation agreed on here.\n\n---\n' +
                `Created from a [conversation with the agent](/agent-chat/${idConversation})`
        );
        // A saved draft is an ordinary issue: nobody, the agent included, works on it.
        expect(saved.assignedTo ?? null).toBeNull();
    });
});
