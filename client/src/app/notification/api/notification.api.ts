import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { Notification } from '../model/notification.model';

export interface NotificationListParams {
    idProject?: number;
    unread?: boolean;
    limit?: number;
    offset?: number;
}

@Injectable({ providedIn: 'root' })
export class NotificationApi {
    private http = inject(HttpClient);

    public list(params: NotificationListParams = {}): Observable<Notification[]> {
        const queryParams: Record<string, string> = {};
        if (params.idProject != null) {
            queryParams['idProject'] = String(params.idProject);
        }
        if (params.unread != null) {
            queryParams['unread'] = String(params.unread);
        }
        if (params.limit != null) {
            queryParams['limit'] = String(params.limit);
        }
        if (params.offset != null) {
            queryParams['offset'] = String(params.offset);
        }
        return this.http.get<Notification[]>('/api/private/notification', { params: queryParams });
    }

    public markRead(idNotification: number): Observable<void> {
        return this.http.put<void>(`/api/private/notification/${idNotification}/read`, {});
    }

    public markAllRead(idProject?: number): Observable<void> {
        const body = idProject != null ? { idProject } : {};
        return this.http.post<void>('/api/private/notification/read', body);
    }

    public delete(idNotification: number): Observable<void> {
        return this.http.delete<void>(`/api/private/notification/${idNotification}`);
    }
}
