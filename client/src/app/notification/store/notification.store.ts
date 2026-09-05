import { Injectable, computed, inject, signal } from '@angular/core';
import { Subscription } from 'rxjs';
import { NotificationApi } from '../api/notification.api';
import { Notification, NotificationGroup } from '../model/notification.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { NotificationType } from '../model/notification-type.enum';

function groupByProject(notifications: Notification[]): NotificationGroup[] {
    const map = new Map<number | null, NotificationGroup>();

    for (const notification of notifications) {
        const key = notification.idProject ?? null;
        if (!map.has(key)) {
            map.set(key, {
                idProject: key,
                projectName: notification.projectName ?? '',
                projectColor: notification.projectColor ?? '#6b7280',
                unreadCount: 0,
                notifications: []
            });
        }
        const group = map.get(key)!;
        group.notifications.push(notification);
        if (!notification.isRead) {
            group.unreadCount++;
        }
    }

    const groups = Array.from(map.values());
    return groups.sort((a, b) => {
        if (a.idProject === null) {
            return -1;
        }
        if (b.idProject === null) {
            return 1;
        }
        return a.projectName.localeCompare(b.projectName);
    });
}

@Injectable({ providedIn: 'root' })
export class NotificationStore {
    private readonly notifApi = inject(NotificationApi);
    private readonly sNotice = inject(NoticeService);

    private notifications = signal<Notification[]>([]);
    private wsSubscription: Subscription | null = null;
    private started = false;

    public readonly all = this.notifications.asReadonly();

    public readonly unreadCount = computed(
        () => this.notifications().filter(n => !n.isRead).length
    );

    public readonly groupedByProject = computed<NotificationGroup[]>(() =>
        groupByProject(this.notifications())
    );

    public readonly mentionUnreadCount = computed(
        () =>
            this.notifications().filter(n => n.type === NotificationType.Mention && !n.isRead)
                .length
    );

    /** Auth-gated bootstrap: fetch the initial list and subscribe to the
     *  websocket feed so notifications work regardless of which components
     *  are on screen. Idempotent — safe to call from both AppComponent
     *  (boot) and SessionService (post-login). */
    public init(): void {
        if (this.started) {
            return;
        }
        this.started = true;
        this.notifApi.list({ limit: 50 }).subscribe(notifications => {
            this.load(notifications);
        });
        this.wsSubscription = this.sNotice.notification$.subscribe(notice => {
            this.prepend(notice.payload);
        });
    }

    /** Tear down the websocket feed — for test cleanup and completeness. */
    public destroy(): void {
        this.wsSubscription?.unsubscribe();
        this.wsSubscription = null;
        this.started = false;
    }

    public load(notifications: Notification[]): void {
        this.notifications.set(notifications);
    }

    public prepend(notification: Notification): void {
        this.notifications.update(current => [notification, ...current]);
    }

    public markRead(idNotification: number): void {
        this.notifications.update(current =>
            current.map(n => (n.idNotification === idNotification ? { ...n, isRead: true } : n))
        );
    }

    public markAllRead(idProject?: number): void {
        this.notifications.update(current =>
            current.map(n =>
                idProject == null || n.idProject === idProject ? { ...n, isRead: true } : n
            )
        );
    }

    public remove(idNotification: number): void {
        this.notifications.update(current =>
            current.filter(n => n.idNotification !== idNotification)
        );
    }
}
