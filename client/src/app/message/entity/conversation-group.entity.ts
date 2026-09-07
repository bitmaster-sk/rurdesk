import { Conversation } from './conversation.entity';

export interface ConversationGroup {
    name?: string;
    /** Translation key that is resolved in the template via `| translate`. */
    nameKey?: string;
    icon: string;
    conversations: Conversation[];
}
