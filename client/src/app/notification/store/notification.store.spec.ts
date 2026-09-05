import { NotificationStore } from './notification.store';
import { Notification } from '../model/notification.model';
import { NotificationType } from '../model/notification-type.enum';

function notif(overrides: Partial<Notification>): Notification {
    return {
        idNotification: 1,
        type: NotificationType.Comment,
        isRead: false,
        createdAt: '2026-04-01T00:00:00Z',
        ...overrides
    };
}

describe('NotificationStore', () => {
    let store: NotificationStore;

    beforeEach(() => {
        store = new NotificationStore();
    });

    it('counts unread notifications', () => {
        store.load([
            notif({ idNotification: 1, isRead: false }),
            notif({ idNotification: 2, isRead: true }),
            notif({ idNotification: 3, isRead: false })
        ]);
        expect(store.unreadCount()).toBe(2);
    });

    it('counts unread mentions by type', () => {
        store.load([
            notif({ idNotification: 1, type: NotificationType.Mention, isRead: false }),
            notif({ idNotification: 2, type: NotificationType.Mention, isRead: true }),
            notif({ idNotification: 3, type: NotificationType.TeamJoined, isRead: false })
        ]);
        expect(store.mentionUnreadCount()).toBe(1);
    });

    it('groups by project with the no-project group first, then alphabetical', () => {
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
        store.load([
            notif({ idNotification: 1, idProject: 1, projectName: 'Alpha', isRead: false }),
            notif({ idNotification: 2, idProject: 1, projectName: 'Alpha', isRead: false }),
            notif({ idNotification: 3, idProject: 1, projectName: 'Alpha', isRead: true })
        ]);
        expect(store.groupedByProject()[0].unreadCount).toBe(2);
    });

    it('markRead flips a single notification', () => {
        store.load([
            notif({ idNotification: 1, isRead: false }),
            notif({ idNotification: 2, isRead: false })
        ]);
        store.markRead(1);
        expect(store.unreadCount()).toBe(1);
    });

    it('markAllRead clears everything, or only a given project', () => {
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
        store.load([notif({ idNotification: 1 })]);
        store.prepend(notif({ idNotification: 2 }));
        expect(store.all().map(n => n.idNotification)).toEqual([2, 1]);

        store.remove(2);
        expect(store.all().map(n => n.idNotification)).toEqual([1]);
    });
});
