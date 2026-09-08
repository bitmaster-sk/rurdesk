import { Injector, runInInjectionContext } from '@angular/core';
import { EMPTY } from 'rxjs';
import { MessageUnreadStore } from './message-unread.store';
import { MessageApi } from '../api/message.api.service';
import { MessageRecipientType } from '../constant/message-recipient-type.enum';
import { MessageKeyConverter } from '../converter/message-key.converter';
import { Message } from '../model/message.model';

function buildStore(): MessageUnreadStore {
    const injector = Injector.create({
        providers: [{ provide: MessageApi, useValue: { loadUnread$: () => EMPTY } }]
    });
    return runInInjectionContext(injector, () => new MessageUnreadStore());
}

function userMessage(idRecipient: number, idUser: number): Message {
    return {
        idRecipient,
        idMessageRecipientType: MessageRecipientType.user,
        creator: { idUser },
        createdAt: new Date()
    } as unknown as Message;
}

describe('MessageUnreadStore', () => {
    it('groups pushed messages by recipient|creator|type key', () => {
        const store = buildStore();
        const key = MessageKeyConverter.toUnreadKey(5, 10, MessageRecipientType.user);
        let unread = new Map<string, Message[]>();
        store.unread$.subscribe(map => (unread = map));

        store.push([userMessage(5, 10), userMessage(5, 10)]);
        expect(unread.get(key)).toHaveLength(2);

        store.remove(5, 10, MessageRecipientType.user);
        expect(unread.has(key)).toBe(false);
    });
});
