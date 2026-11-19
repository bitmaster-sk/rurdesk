import { Message } from 'src/app/message/model/message.model';
import { AgentConversationAuthor } from '../constants/agent-conversation-author.enum';
import { AgentConversationMessageKind } from '../constants/agent-conversation-message-kind.enum';

/** One line of the conversation view. */
export interface AgentChatEntry {
    idMessage: number;
    author: AgentConversationAuthor;
    kind: AgentConversationMessageKind;
    /** What `app-message-view` renders; null for a system line. */
    message: Message | null;
    /** Translation key of a system line, null for a user or agent message. */
    systemKey: string | null;
    systemParams: Record<string, string>;
    /** The agent turn behind the message, whose thinking can be replayed. */
    idTask: number | null;
    hasThinking: boolean;
    /** Sent by the owner but not confirmed by the server yet. */
    isPending: boolean;
}
