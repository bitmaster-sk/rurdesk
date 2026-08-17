import { HttpClient } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { MessageService } from './message.service';
import { MessageRecipientType } from './constant/message-recipient-type.enum';
import { Message } from './model/message.model';

function buildService(http: HttpClient): MessageService {
    const injector = Injector.create({ providers: [{ provide: HttpClient, useValue: http }] });
    return runInInjectionContext(injector, () => new MessageService());
}

function userMessage(idRecipient: number, idUser: number): Message {
    return {
        idRecipient,
        idMessageRecipientType: MessageRecipientType.user,
        creator: { idUser },
        createdAt: new Date()
    } as unknown as Message;
}

describe('MessageService.insertMessage', () => {
    it('includes anchor fields in the body when an anchor is given', () => {
        const post = vi.fn().mockReturnValue(of({ createdAt: '2026-01-01T00:00:00Z' }));
        const service = new MessageService({ post } as unknown as HttpClient);

        service
            .insertMessage(5, MessageRecipientType.user, 'hi', {
                idParentMessage: 1,
                anchorLineStart: 2,
                anchorLineEnd: 3
            })
            .subscribe();

        const [url, body] = post.mock.calls[0] as [string, Record<string, unknown>];
        expect(url).toBe('/api/private/message');
        expect(body).toMatchObject({
            idRecipient: 5,
            message: 'hi',
            idParentMessage: 1,
            anchorLineStart: 2,
            anchorLineEnd: 3
        });
    });
});

describe('MessageService unread map', () => {
    it('groups pushed messages by recipient|creator|type key', () => {
        const service = new MessageService({} as unknown as HttpClient);
        const key = `5|10|${MessageRecipientType.user}`;

        service.unreadPush([userMessage(5, 10), userMessage(5, 10)]);
        expect(service.Unread.getValue().get(key)).toHaveLength(2);

        service.unreadRemove(5, 10, MessageRecipientType.user);
        expect(service.Unread.getValue().has(key)).toBe(false);
    });
});
