import { User } from 'src/app/auth/model/user.model';
import { MessageKind } from '../constant/message-kind.enum';
import { MessageRecipientType } from '../constant/message-recipient-type.enum';

export interface MessageAnchor {
    idParentMessage: number;
    parentVersion: number;
    anchorLineStart: number;
    anchorLineEnd: number;
    isOutdated: boolean;
    /** Parent source of the anchored lines at creation, cut to 500 characters. */
    anchorText: string;
    /** Exact selected text; absent when the whole line range is meant. */
    quote?: string;
    /** Thread root; absent on the root itself. */
    idThreadMessage?: number;
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
