import { MessageRecipientType } from '../constant/message-recipient-type.enum';

export abstract class MessageKeyConverter {
    public static toUnreadKey(
        idRecipient: number,
        idCreator: number | null,
        idMessageRecipientType: MessageRecipientType
    ): string {
        return `${idRecipient}|${idCreator}|${idMessageRecipientType}`;
    }
}
