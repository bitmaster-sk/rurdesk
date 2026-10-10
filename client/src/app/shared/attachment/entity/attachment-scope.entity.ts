import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';

export interface AttachmentScope {
    idMessageRecipientType: MessageRecipientType;
    idRecipient: number;
}
