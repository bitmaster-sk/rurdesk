import { AnchorThread } from 'src/app/issue/entity/anchor-thread.entity';
import { Message } from 'src/app/message/model/message.model';

export abstract class AnchorThreadConverter {
    /**
     * Groups anchored replies into threads by their root, keyed by parent message. Position and
     * state come from the root; a thread whose root is not loaded falls back to its oldest reply.
     */
    public static toThreadsByParent(messages: Message[]): Map<number, AnchorThread[]> {
        const byThread = new Map<number, Message[]>();
        for (const message of messages) {
            if (!message.anchor) {
                continue;
            }
            const idThread = AnchorThreadConverter.threadOf(message);
            byThread.set(idThread, [...(byThread.get(idThread) ?? []), message]);
        }

        const byParent = new Map<number, AnchorThread[]>();
        for (const [idThread, replies] of byThread) {
            const sorted = [...replies].sort(
                (a, b) => a.createdAt.getTime() - b.createdAt.getTime()
            );
            const root = sorted.find(reply => reply.idMessage === idThread) ?? sorted[0];
            const anchor = root.anchor!;
            const thread: AnchorThread = {
                idThread,
                idParentMessage: anchor.idParentMessage,
                lineStart: anchor.anchorLineStart,
                lineEnd: anchor.anchorLineEnd,
                quote: anchor.quote,
                anchorText: anchor.anchorText,
                isOutdated: anchor.isOutdated,
                replies: sorted
            };
            byParent.set(anchor.idParentMessage, [
                ...(byParent.get(anchor.idParentMessage) ?? []),
                thread
            ]);
        }
        for (const threads of byParent.values()) {
            threads.sort((a, b) => a.lineStart - b.lineStart || a.idThread - b.idThread);
        }
        return byParent;
    }

    public static threadOf(message: Message): number {
        return message.anchor?.idThreadMessage ?? message.idMessage;
    }
}
