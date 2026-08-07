import { MessageRecipientType } from '../constant/message-recipient-type.enum';

export interface Conversation {
    idRecipient: number;
    idMessageRecipientType: MessageRecipientType;
    name: string;
    idCreator: number | null;
    url: (string | number)[];
    unreadKey: string;
}
