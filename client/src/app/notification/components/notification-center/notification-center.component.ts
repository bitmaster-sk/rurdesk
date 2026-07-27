import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { NotificationApi } from '../../api/notification.api';
import { NotificationType } from '../../model/notification-type.enum';
import { Notification, NotificationGroup } from '../../model/notification.model';
import { NotificationStore } from '../../store/notification.store';

type NotifFilter =
    | { type: 'all' }
    | { type: 'mentions' }
    | { type: 'general' }
    | { type: 'project'; idProject: number };

@Component({
    selector: 'app-notification-center',
    templateUrl: './notification-center.component.html',
    styleUrls: ['./notification-center.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class NotificationCenterComponent {
    private store = inject(NotificationStore);
    private notifApi = inject(NotificationApi);

    protected readonly NotificationType = NotificationType;

    private _activeFilter = signal<NotifFilter>({ type: 'all' });
    protected readonly activeFilter = this._activeFilter.asReadonly();

    protected sidebarGroups = this.store.groupedByProject;
    protected totalUnread = this.store.unreadCount;
    protected mentionCount = this.store.mentionUnreadCount;

    protected filteredNotifications = computed<Notification[]>(() => {
        const filter = this._activeFilter();
        const all = this.store.all();
        switch (filter.type) {
            case 'all':
                return all;
            case 'mentions':
                return all.filter(n => n.type === NotificationType.Mention);
            case 'general':
                return all.filter(n => !n.idProject);
            case 'project':
                return all.filter(n => n.idProject === filter.idProject);
        }
    });

    protected filterTitle = computed<string>(() => {
        const filter = this._activeFilter();
        switch (filter.type) {
            case 'all':
                return 'NOTIFICATION.FILTER.ALL';
            case 'mentions':
                return 'NOTIFICATION.FILTER.MENTIONS';
            case 'general':
                return 'NOTIFICATION.FILTER.GENERAL';
            case 'project':
                return (
                    this.store
                        .groupedByProject()
                        .find(
                            (g: NotificationGroup) =>
                                g.idProject ===
                                (filter as { type: 'project'; idProject: number }).idProject
                        )?.projectName ?? ''
                );
        }
    });

    protected activeFilterProjectId = computed<number | null>(() => {
        const f = this._activeFilter();
        return f.type === 'project' ? f.idProject : null;
    });

    protected onSetFilter(filter: NotifFilter): void {
        this._activeFilter.set(filter);
    }

    protected onMarkAllAsRead(): void {
        const filter = this._activeFilter();
        const idProject = filter.type === 'project' ? filter.idProject : undefined;
        this.notifApi.markAllRead(idProject).subscribe(() => {
            this.store.markAllRead(idProject);
        });
    }

    protected onMarkAsRead(notification: Notification): void {
        this.notifApi.markRead(notification.idNotification).subscribe(() => {
            this.store.markRead(notification.idNotification);
        });
    }

    protected onDismiss(notification: Notification): void {
        this.notifApi.delete(notification.idNotification).subscribe(() => {
            this.store.remove(notification.idNotification);
        });
    }
}
