import { Injector, runInInjectionContext } from '@angular/core';
import { EMPTY, Subject } from 'rxjs';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { MessageUnreadStore } from './message-unread.store';
import { MessageApi } from '../api/message.api.service';
import { MessageRecipientType } from '../constant/message-recipient-type.enum';
import { MessageKeyConverter } from '../converter/message-key.converter';
import { Message } from '../model/message.model';

function buildStore(notices = new Subject<Notice<Message>>()): MessageUnreadStore {
    const injector = Injector.create({
        providers: [
            { provide: MessageApi, useValue: { loadUnread$: () => EMPTY } },
            { provide: NoticeService, useValue: { Message: notices } }
        ]
    });
    return runInInjectionContext(injector, () => new MessageUnreadStore());
}

function notice(action: NoticeAction, payload: Message): Notice<Message> {
    return { action, payload } as unknown as Notice<Message>;
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

    it('counts a message notice without any page open', () => {
        const notices = new Subject<Notice<Message>>();
        const store = buildStore(notices);
        const key = MessageKeyConverter.toUnreadKey(5, 10, MessageRecipientType.user);
        let unread = new Map<string, Message[]>();
        store.unread$.subscribe(map => (unread = map));

        notices.next(notice(NoticeAction.Create, userMessage(5, 10)));
        notices.next(notice(NoticeAction.Update, userMessage(5, 10)));
        notices.next(
            notice(NoticeAction.Create, {
                ...userMessage(5, 10),
                idMessageRecipientType: MessageRecipientType.issue
            })
        );

        expect(unread.get(key)).toHaveLength(1);
        expect(unread.size).toBe(1);
    });
});
