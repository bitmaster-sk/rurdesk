import { Injector, runInInjectionContext } from '@angular/core';
import { Subject, of } from 'rxjs';
import { NotificationApi } from '../api/notification.api';
import { NotificationStore } from './notification.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { Notification } from '../model/notification.model';
import { NotificationType } from '../model/notification-type.enum';

function notif(overrides: Partial<Notification>): Notification {
    return {
        idNotification: 1,
        type: NotificationType.Comment,
        isRead: false,
        createdAt: new Date('2026-04-01T00:00:00Z'),
        ...overrides
    };
}

function buildStore(listReturn = of<Notification[]>([])): {
    store: NotificationStore;
    list$: ReturnType<typeof vi.fn>;
    notification$: Subject<Notice<Notification>>;
} {
    const list$ = vi.fn().mockReturnValue(listReturn);
    const notification$ = new Subject<Notice<Notification>>();
    const injector = Injector.create({
        providers: [
            { provide: NotificationApi, useValue: { list: list$ } },
            { provide: NoticeService, useValue: { notification$ } }
        ]
    });
    const store = runInInjectionContext(injector, () => new NotificationStore());
    return { store, list$, notification$ };
}

describe('NotificationStore', () => {
    it('counts unread notifications', () => {
        const { store } = buildStore();
        store.load([
            notif({ idNotification: 1, isRead: false }),
            notif({ idNotification: 2, isRead: true }),
            notif({ idNotification: 3, isRead: false })
        ]);
        expect(store.unreadCount()).toBe(2);
    });

    it('counts unread mentions by type', () => {
        const { store } = buildStore();
        store.load([
            notif({ idNotification: 1, type: NotificationType.Mention, isRead: false }),
            notif({ idNotification: 2, type: NotificationType.Mention, isRead: true }),
            notif({ idNotification: 3, type: NotificationType.TeamJoined, isRead: false })
        ]);
        expect(store.mentionUnreadCount()).toBe(1);
    });

    it('groups by project with the no-project group first, then alphabetical', () => {
        const { store } = buildStore();
        store.load([
            notif({ idNotification: 1, idProject: 2, projectName: 'Zeta', isRead: false }),
            notif({ idNotification: 2, idProject: 1, projectName: 'Alpha', isRead: true }),
            notif({ idNotification: 3, projectName: undefined }) // no project
        ]);
        const groups = store.groupedByProject();
        expect(groups.map(g => g.idProject)).toEqual([null, 1, 2]);
        expect(groups[2].projectName).toBe('Zeta');
    });

    it('tracks unread count per group', () => {
        const { store } = buildStore();
        store.load([
            notif({ idNotification: 1, idProject: 1, projectName: 'Alpha', isRead: false }),
            notif({ idNotification: 2, idProject: 1, projectName: 'Alpha', isRead: false }),
            notif({ idNotification: 3, idProject: 1, projectName: 'Alpha', isRead: true })
        ]);
        expect(store.groupedByProject()[0].unreadCount).toBe(2);
    });

    it('markRead flips a single notification', () => {
        const { store } = buildStore();
        store.load([
            notif({ idNotification: 1, isRead: false }),
            notif({ idNotification: 2, isRead: false })
        ]);
        store.markRead(1);
        expect(store.unreadCount()).toBe(1);
    });

    it('markAllRead clears everything, or only a given project', () => {
        const { store } = buildStore();
        store.load([
            notif({ idNotification: 1, idProject: 1, isRead: false }),
            notif({ idNotification: 2, idProject: 2, isRead: false })
        ]);
        store.markAllRead(1);
        expect(store.unreadCount()).toBe(1); // project 2 still unread

        store.markAllRead();
        expect(store.unreadCount()).toBe(0);
    });

    it('prepend adds to the front and remove deletes by id', () => {
        const { store } = buildStore();
        store.load([notif({ idNotification: 1 })]);
        store.prepend(notif({ idNotification: 2 }));
        expect(store.all().map(n => n.idNotification)).toEqual([2, 1]);

        store.remove(2);
        expect(store.all().map(n => n.idNotification)).toEqual([1]);
    });

    describe('init', () => {
        it('loads the initial list from the API', () => {
            const fetched = [
                notif({ idNotification: 10, isRead: true }),
                notif({ idNotification: 11, isRead: false })
            ];
            const { store, list$ } = buildStore(of(fetched));

            store.init();

            expect(list$).toHaveBeenCalledWith({ limit: 50 });
            expect(store.all().map(n => n.idNotification)).toEqual([10, 11]);
        });

        it('subscribes to websocket and prepends incoming notices', () => {
            const initial = notif({ idNotification: 1 });
            const { store, notification$ } = buildStore(of([initial]));

            store.init();
            expect(store.all().map(n => n.idNotification)).toEqual([1]);

            const incoming = notif({ idNotification: 2 });
            notification$.next({
                subject: NoticeSubject.Notification,
                action: NoticeAction.Create,
                payload: incoming
            });

            expect(store.all().map(n => n.idNotification)).toEqual([2, 1]);
        });

        it('is idempotent — second call does not re-fetch or double-subscribe', () => {
            const fetched = [notif({ idNotification: 1 })];
            const { store, list$, notification$ } = buildStore(of(fetched));

            store.init();
            store.init();

            expect(list$).toHaveBeenCalledTimes(1);

            // only one prepend happens (one subscription, not two)
            notification$.next({
                subject: NoticeSubject.Notification,
                action: NoticeAction.Create,
                payload: notif({ idNotification: 2 })
            });
            expect(store.all().map(n => n.idNotification)).toEqual([2, 1]);
        });
    });

    describe('destroy', () => {
        it('stops the websocket feed', () => {
            const { store, notification$ } = buildStore(of([]));

            store.init();
            store.destroy();

            notification$.next({
                subject: NoticeSubject.Notification,
                action: NoticeAction.Create,
                payload: notif({ idNotification: 99 })
            });

            expect(store.all()).toEqual([]);
        });

        it('allows init to restart the feed after destroy', () => {
            const fetched = [notif({ idNotification: 1 })];
            const { store, notification$ } = buildStore(of(fetched));

            store.init();
            store.destroy();
            store.init();

            notification$.next({
                subject: NoticeSubject.Notification,
                action: NoticeAction.Create,
                payload: notif({ idNotification: 2 })
            });

            expect(store.all().map(n => n.idNotification)).toEqual([2, 1]);
        });
    });
});
