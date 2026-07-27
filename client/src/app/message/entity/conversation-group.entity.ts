import { Conversation } from './conversation.entity';

export interface ConversationGroup {
    name: string;
    icon: string;
    conversations: Conversation[];
}
