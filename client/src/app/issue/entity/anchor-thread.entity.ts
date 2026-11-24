import { Message } from 'src/app/message/model/message.model';

/** Replies anchored to one range of a parent message, oldest first. */
export interface AnchorThread {
    /** The thread root: its first reply. */
    idThread: number;
    idParentMessage: number;
    lineStart: number;
    lineEnd: number;
    quote?: string;
    anchorText: string;
    isOutdated: boolean;
    replies: Message[];
}

/** A thread being started from a text selection, before its first reply is sent. */
export interface AnchorDraft {
    idParentMessage: number;
    lineStart: number;
    lineEnd: number;
    quote: string;
}
