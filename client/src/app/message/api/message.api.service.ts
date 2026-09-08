import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { MessageRecipientType } from '../constant/message-recipient-type.enum';
import { MessageConverter } from '../converter/message.converter';
import { Message } from '../model/message.model';

@Injectable({
    providedIn: 'root'
})
export class MessageApi {
    private readonly http = inject(HttpClient);

    public load$(idRecipient: number, idMessageRecipientType: number): Observable<Message[]> {
        let params = new HttpParams();
        params = params.append('idRecipient', idRecipient.toString());
        params = params.append('idMessageRecipientType', idMessageRecipientType.toString());
        return this.http
            .get<Message[]>(`/api/private/message/`, { params })
            .pipe(map(msgs => MessageConverter.toMessages(msgs)));
    }

    public insert$(
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
            .pipe(map(msg => MessageConverter.toMessage(msg)));
    }

    public markRead$(
        idRecipient: number,
        idMessageRecipientType: MessageRecipientType
    ): Observable<void> {
        return this.http.post<void>('/api/private/message/read', {
            idRecipient,
            idMessageRecipientType
        });
    }

    public update$(idMessage: number, newText: string): Observable<Message> {
        return this.http
            .patch<Message>(`/api/private/message/${idMessage}`, { message: newText })
            .pipe(map(msg => MessageConverter.toMessage(msg)));
    }

    public loadUnread$(): Observable<Message[]> {
        return this.http
            .get<Message[]>('/api/private/message/unread')
            .pipe(map(msgs => MessageConverter.toMessages(msgs)));
    }
}
