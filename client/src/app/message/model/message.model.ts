import { User } from 'src/app/auth/model/user.model';
import { MessageKind } from '../constant/message-kind.enum';
import { MessageRecipientType } from '../constant/message-recipient-type.enum';

export interface MessageAnchor {
    idParentMessage: number;
    parentVersion: number;
    anchorLineStart: number;
    anchorLineEnd: number;
    isOutdated: boolean;
}

export interface Message {
    idMessage: number;
    message: string;
    messageKind: MessageKind;
    createdAt: Date;
    updatedAt?: Date;
    isRead: boolean;
    creator: User;
    idRecipient: number;
    idMessageRecipientType: MessageRecipientType;
    version: number;
    anchor: MessageAnchor | null;
}
