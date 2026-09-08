import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { MessageApi } from '../api/message.api.service';
import { MessageRecipientType } from '../constant/message-recipient-type.enum';
import { MessageKeyConverter } from '../converter/message-key.converter';
import { Message } from '../model/message.model';

@Injectable({
    providedIn: 'root'
})
export class MessageUnreadStore {
    private readonly messageApi = inject(MessageApi);

    private readonly _unread$ = new BehaviorSubject<Map<string, Message[]>>(
        new Map<string, Message[]>()
    );

    public readonly unread$: Observable<Map<string, Message[]>> = this._unread$.asObservable();

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
