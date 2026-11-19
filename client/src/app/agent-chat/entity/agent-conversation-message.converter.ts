import { User } from 'src/app/auth/model/user.model';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';
import { Message } from 'src/app/message/model/message.model';
import { AgentConversationAuthor } from '../constants/agent-conversation-author.enum';
import { AgentConversationMessageKind } from '../constants/agent-conversation-message-kind.enum';
import { AgentConversationMessage } from '../model/agent-conversation.model';
import { AgentChatEntry } from './agent-chat-entry.entity';

const SHORT_COMMIT = 7;

// The body of an error line is the reason the turn failed with.
const ERROR_KEYS: Record<string, string> = {
    stage_not_submitted: 'AGENT_CHAT.SYSTEM.STAGE_NOT_SUBMITTED',
    gateway_restart: 'AGENT_CHAT.SYSTEM.GATEWAY_RESTART',
    gateway_restart_exhausted: 'AGENT_CHAT.SYSTEM.GATEWAY_RESTART',
    dispatch_failed: 'AGENT_CHAT.SYSTEM.DISPATCH_FAILED',
    heartbeat_stale: 'AGENT_CHAT.SYSTEM.HEARTBEAT_STALE',
    agent_reported_error: 'AGENT_CHAT.SYSTEM.AGENT_ERROR'
};

export abstract class AgentConversationMessageConverter {
    public static toEntries(
        messages: AgentConversationMessage[],
        owner: User,
        agent: User
    ): AgentChatEntry[] {
        return messages.map(message => this.toEntry(message, owner, agent));
    }

    // A message the owner just sent carries a negative id until the server
    // returns the stored one.
    public static toEntry(
        source: AgentConversationMessage,
        owner: User,
        agent: User
    ): AgentChatEntry {
        const isSystem = source.author === AgentConversationAuthor.System;
        return {
            idMessage: source.idMessage,
            author: source.author,
            kind: source.kind,
            message: isSystem
                ? null
                : this.toMessage(
                      source,
                      source.author === AgentConversationAuthor.Agent ? agent : owner
                  ),
            systemKey: isSystem ? this.toSystemKey(source) : null,
            systemParams: isSystem ? { detail: this.toSystemDetail(source) } : {},
            idTask: source.idTask,
            hasThinking: source.hasThinking,
            isPending: source.idMessage < 0
        };
    }

    // The Message shape belongs to the project and team chat; a conversation has
    // no recipient of its own there, so it reads as a direct conversation.
    private static toMessage(source: AgentConversationMessage, creator: User): Message {
        return {
            idMessage: source.idMessage,
            message: source.body,
            messageKind: MessageKind.Comment,
            createdAt: new Date(source.createdAt),
            isRead: true,
            creator,
            idRecipient: source.idConversation,
            idMessageRecipientType: MessageRecipientType.user,
            version: 1,
            anchor: null
        };
    }

    private static toSystemKey(source: AgentConversationMessage): string {
        switch (source.kind) {
            case AgentConversationMessageKind.Error:
                return ERROR_KEYS[source.body] ?? 'AGENT_CHAT.SYSTEM.ERROR';
            case AgentConversationMessageKind.CodeRefreshed:
                return 'AGENT_CHAT.SYSTEM.CODE_REFRESHED';
            default:
                return 'AGENT_CHAT.SYSTEM.OTHER';
        }
    }

    private static toSystemDetail(source: AgentConversationMessage): string {
        return source.kind === AgentConversationMessageKind.CodeRefreshed
            ? source.body.slice(0, SHORT_COMMIT)
            : source.body;
    }
}
