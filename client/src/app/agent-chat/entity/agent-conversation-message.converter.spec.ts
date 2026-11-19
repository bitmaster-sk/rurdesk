import { User } from 'src/app/auth/model/user.model';
import { AgentConversationMessage } from '../model/agent-conversation.model';
import { AgentConversationMessageConverter } from './agent-conversation-message.converter';
import { AgentConversationAuthor } from '../constants/agent-conversation-author.enum';
import { AgentConversationMessageKind } from '../constants/agent-conversation-message-kind.enum';

const owner: User = { idUser: 1, name: 'Ada', email: 'ada@x.io', colorAvatarBg: '#123' };
const agent: User = {
    idUser: 9,
    name: 'Agent 003',
    email: 'agent@x.io',
    colorAvatarBg: '#456',
    isAgent: true
};

function message(overrides: Partial<AgentConversationMessage>): AgentConversationMessage {
    return {
        idMessage: 10,
        idConversation: 4,
        author: AgentConversationAuthor.User,
        kind: AgentConversationMessageKind.Text,
        body: 'Where is the worktree removed?',
        idUser: 1,
        idTask: null,
        hasThinking: false,
        createdAt: '2026-11-16T10:41:00Z',
        ...overrides
    };
}

describe('AgentConversationMessageConverter.toEntry', () => {
    it('shows the owner as the author of a user message', () => {
        const entry = AgentConversationMessageConverter.toEntry(message({}), owner, agent);

        expect(entry.author).toBe('user');
        expect(entry.message!.creator).toBe(owner);
        expect(entry.message!.message).toBe('Where is the worktree removed?');
        expect(entry.message!.idMessage).toBe(10);
        expect(entry.message!.createdAt).toEqual(new Date('2026-11-16T10:41:00Z'));
        expect(entry.message!.isRead).toBe(true);
        expect(entry.systemKey).toBeNull();
        expect(entry.isPending).toBe(false);
    });

    it('shows the agent as the author of an agent reply and keeps its thinking reference', () => {
        const entry = AgentConversationMessageConverter.toEntry(
            message({
                author: AgentConversationAuthor.Agent,
                idUser: 9,
                idTask: 77,
                hasThinking: true,
                body: 'In gc.go'
            }),
            owner,
            agent
        );

        expect(entry.message!.creator).toBe(agent);
        expect(entry.message!.message).toBe('In gc.go');
        expect(entry.idTask).toBe(77);
        expect(entry.hasThinking).toBe(true);
    });

    it.each([
        ['stage_not_submitted', 'AGENT_CHAT.SYSTEM.STAGE_NOT_SUBMITTED'],
        ['gateway_restart', 'AGENT_CHAT.SYSTEM.GATEWAY_RESTART'],
        ['gateway_restart_exhausted', 'AGENT_CHAT.SYSTEM.GATEWAY_RESTART'],
        ['dispatch_failed', 'AGENT_CHAT.SYSTEM.DISPATCH_FAILED'],
        ['heartbeat_stale', 'AGENT_CHAT.SYSTEM.HEARTBEAT_STALE'],
        ['agent_reported_error', 'AGENT_CHAT.SYSTEM.AGENT_ERROR']
    ])('explains a turn that failed with %s', (reason, key) => {
        const entry = AgentConversationMessageConverter.toEntry(
            message({
                author: AgentConversationAuthor.System,
                kind: AgentConversationMessageKind.Error,
                body: reason,
                idUser: null
            }),
            owner,
            agent
        );

        expect(entry.message).toBeNull();
        expect(entry.systemKey).toBe(key);
    });

    it('names an unknown failure reason in a generic line', () => {
        const entry = AgentConversationMessageConverter.toEntry(
            message({
                author: AgentConversationAuthor.System,
                kind: AgentConversationMessageKind.Error,
                body: 'provider_rate_limited',
                idUser: null
            }),
            owner,
            agent
        );

        expect(entry.systemKey).toBe('AGENT_CHAT.SYSTEM.ERROR');
        expect(entry.systemParams).toEqual({ detail: 'provider_rate_limited' });
    });

    it('turns a code refresh into a muted line with the short commit', () => {
        const entry = AgentConversationMessageConverter.toEntry(
            message({
                author: AgentConversationAuthor.System,
                kind: AgentConversationMessageKind.CodeRefreshed,
                body: '0123456789abcdef0123',
                idUser: null
            }),
            owner,
            agent
        );

        expect(entry.message).toBeNull();
        expect(entry.systemKey).toBe('AGENT_CHAT.SYSTEM.CODE_REFRESHED');
        expect(entry.systemParams).toEqual({ detail: '0123456' });
    });

    it('marks a message the server has not stored yet as pending', () => {
        const entry = AgentConversationMessageConverter.toEntry(
            message({ idMessage: -1 }),
            owner,
            agent
        );

        expect(entry.isPending).toBe(true);
    });
});
