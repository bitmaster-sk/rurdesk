import { NotificationBody } from './notification-body.model';
import { NotificationType } from './notification-type.enum';

export interface Notification {
    idNotification: number;
    type: NotificationType;
    idProject?: number;
    projectName?: string;
    projectColor?: string;
    actorName?: string;
    actorAvatarBg?: string;
    refType?: string;
    refId?: string;
    refTitle?: string;
    refPublicId?: number;
    body?: NotificationBody;
    isRead: boolean;
    createdAt: string;
}

export interface NotificationGroup {
    idProject: number | null;
    projectName: string;
    projectColor: string;
    unreadCount: number;
    notifications: Notification[];
}
