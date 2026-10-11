import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';

export interface MessageAttachmentScope {
    idMessageRecipientType: MessageRecipientType;
    idRecipient: number;
}

export interface WikiAttachmentScope {
    idSpace: number;
}

export type AttachmentScope = MessageAttachmentScope | WikiAttachmentScope;
