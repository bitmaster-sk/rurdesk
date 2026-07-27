import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { map, tap } from 'rxjs/operators';
import { MessageRecipientType } from './constant/message-recipient-type.enum';
import { Message } from './model/message.model';

@Injectable({
    providedIn: 'root'
})
export class MessageService {
    public Unread: BehaviorSubject<Map<string, Message[]>> = new BehaviorSubject<
        Map<string, Message[]>
    >(new Map<string, Message[]>());

    // TODO plnit unread v tejto service
    constructor(private http: HttpClient) {}

    public loadMessages(
        idRecipient: number,
        idMessageRecipientType: number
    ): Observable<Message[]> {
        let params = new HttpParams();
        params = params.append('idRecipient', idRecipient.toString());
        params = params.append('idMessageRecipientType', idMessageRecipientType.toString());
        return this.http
            .get<Message[]>(`/api/private/message/`, { params })
            .pipe(map(msgs => this.toMessages(msgs)));
    }

    public insertMessage(
        idRecipient: number,
        idMessageRecipientType: MessageRecipientType,
        message: string,
        anchor?: { idParentMessage: number; anchorLineStart: number; anchorLineEnd: number }
    ): Observable<Message> {
        const body: Record<string, unknown> = { idRecipient, idMessageRecipientType, message };
        if (anchor) {
            body['idParentMessage'] = anchor.idParentMessage;
            body['anchorLineStart'] = anchor.anchorLineStart;
            body['anchorLineEnd'] = anchor.anchorLineEnd;
        }
        return this.http
            .post<Message>('/api/private/message', body)
            .pipe(map(msg => this.toMessage(msg)));
    }

    public insertReadMessage(
        idRecipient: number,
        idMessageRecipientType: MessageRecipientType
    ): Observable<void> {
        return this.http.post<void>('/api/private/message/read', {
            idRecipient,
            idMessageRecipientType
        });
    }

    public updateMessage(idMessage: number, newText: string): Observable<Message> {
        return this.http
            .patch<Message>(`/api/private/message/${idMessage}`, { message: newText })
            .pipe(map(msg => this.toMessage(msg)));
    }

    public loadUnreadMessages(): Observable<Message[]> {
        return this.http.get<Message[]>('/api/private/message/unread').pipe(
            map(msgs => this.toMessages(msgs)),
            tap(msgs => this.unreadPush(msgs))
        );
    }

    public unreadPush(messages: Message[]): void {
        const unreadMap = new Map(this.Unread.getValue());
        messages.forEach(m => {
            const id = `${m.idRecipient}|${m.idMessageRecipientType === MessageRecipientType.user ? m.creator.idUser : null}|${m.idMessageRecipientType}`;
            const unread = unreadMap.get(id);
            if (unread) {
                unread.push(m);
            } else {
                unreadMap.set(id, [m]);
            }
        });
        this.Unread.next(unreadMap);
    }

    public unreadRemove(
        idRecipient: number,
        idCreator: number,
        idMessageRecipientType: MessageRecipientType
    ): void {
        const unreadMap = new Map(this.Unread.getValue());
        const id = `${idRecipient}|${idCreator}|${idMessageRecipientType}`;
        unreadMap.delete(id);
        this.Unread.next(unreadMap);
    }

    private toMessages(msgs: Message[]): Message[] {
        return msgs.map(msg => this.toMessage(msg));
    }

    private toMessage(m: Message): Message {
        m.createdAt = new Date(m.createdAt);
        if (m.updatedAt) {
            m.updatedAt = new Date(m.updatedAt as unknown as string);
        }
        return m;
    }
}
