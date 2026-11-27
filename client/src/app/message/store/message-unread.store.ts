import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, Observable, filter } from 'rxjs';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { MessageApi } from '../api/message.api.service';
import { MessageRecipientType } from '../constant/message-recipient-type.enum';
import { MessageKeyConverter } from '../converter/message-key.converter';
import { Message } from '../model/message.model';

/**
 * Unread chat messages per conversation. A new chat message counts as soon as its notice arrives,
 * whatever page is open; a read conversation is removed by the page that read it.
 */
@Injectable({
    providedIn: 'root'
})
export class MessageUnreadStore {
    private readonly messageApi = inject(MessageApi);
    private readonly sNotice = inject(NoticeService);

    private readonly _unread$ = new BehaviorSubject<Map<string, Message[]>>(
        new Map<string, Message[]>()
    );

    public readonly unread$: Observable<Map<string, Message[]>> = this._unread$.asObservable();

    public constructor() {
        // Lives as long as the app, like the store.
        this.sNotice.Message.pipe(
            filter(
                notice =>
                    notice.payload.idMessageRecipientType !== MessageRecipientType.issue &&
                    notice.action !== NoticeAction.Update
            )
        ).subscribe(notice => this.push([notice.payload]));
    }

    public load(): void {
        this.messageApi.loadUnread$().subscribe(msgs => this.push(msgs));
    }

    public push(messages: Message[]): void {
        const unreadMap = new Map(this._unread$.getValue());
        messages.forEach(m => {
            const id = MessageKeyConverter.toUnreadKey(
                m.idRecipient,
                m.idMessageRecipientType === MessageRecipientType.user ? m.creator.idUser : null,
                m.idMessageRecipientType
            );
            const unread = unreadMap.get(id);
            if (unread) {
                unread.push(m);
            } else {
                unreadMap.set(id, [m]);
            }
        });
        this._unread$.next(unreadMap);
    }

    public remove(
        idRecipient: number,
        idCreator: number | null,
        idMessageRecipientType: MessageRecipientType
    ): void {
        const unreadMap = new Map(this._unread$.getValue());
        unreadMap.delete(
            MessageKeyConverter.toUnreadKey(idRecipient, idCreator, idMessageRecipientType)
        );
        this._unread$.next(unreadMap);
    }
}
