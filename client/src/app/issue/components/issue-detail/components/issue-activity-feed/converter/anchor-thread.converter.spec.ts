import { describe, it, expect } from 'vitest';
import { AnchorThreadConverter } from './anchor-thread.converter';
import { Message, MessageAnchor } from 'src/app/message/model/message.model';

function reply(idMessage: number, minute: number, anchor: Partial<MessageAnchor> | null): Message {
    const partial: Partial<Message> = {
        idMessage,
        createdAt: new Date(2026, 10, 23, 10, minute),
        anchor: anchor && {
            idParentMessage: 1,
            parentVersion: 1,
            anchorLineStart: 3,
            anchorLineEnd: 4,
            isOutdated: false,
            anchorText: 'x',
            ...anchor
        }
    };
    return partial as Message;
}

describe('AnchorThreadConverter.toThreadsByParent', () => {
    it('groups replies under their root, oldest first, with the root position', () => {
        const threads = AnchorThreadConverter.toThreadsByParent([
            reply(1, 0, null),
            reply(5, 3, { idThreadMessage: 2, anchorLineStart: 9, isOutdated: true }),
            reply(2, 1, { quote: 'step' }),
            reply(7, 2, { anchorLineStart: 1, anchorLineEnd: 1 })
        ]).get(1)!;

        expect(threads.map(thread => thread.idThread)).toEqual([7, 2]);
        expect(threads[1]).toMatchObject({
            lineStart: 3,
            lineEnd: 4,
            quote: 'step',
            isOutdated: false
        });
        expect(threads[1].replies.map(message => message.idMessage)).toEqual([2, 5]);
    });

    it('falls back to the oldest reply when the root is not loaded', () => {
        const threads = AnchorThreadConverter.toThreadsByParent([
            reply(6, 5, { idThreadMessage: 2, anchorLineStart: 8, anchorLineEnd: 8 }),
            reply(5, 4, { idThreadMessage: 2, anchorLineStart: 7, anchorLineEnd: 7 })
        ]).get(1)!;

        expect(threads).toHaveLength(1);
        expect(threads[0]).toMatchObject({ idThread: 2, lineStart: 7 });
    });
});
