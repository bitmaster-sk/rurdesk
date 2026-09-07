import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { formatDistanceToNow } from 'date-fns';
import {
    NotificationBodySeverity,
    NotificationBodyState
} from '../../model/notification-body.model';
import { NotificationType } from '../../model/notification-type.enum';
import { Notification } from '../../model/notification.model';

interface TypeChip {
    icon: string;
    colorClass: string;
    labelKey: string;
}

const TYPE_CHIPS: Record<NotificationType, TypeChip> = {
    [NotificationType.Comment]: {
        icon: 'message',
        colorClass: 'notif-card-badge__comment',
        labelKey: 'NOTIFICATION.TYPE.COMMENT'
    },
    [NotificationType.Mention]: {
        icon: 'at',
        colorClass: 'notif-card-badge__mention',
        labelKey: 'NOTIFICATION.TYPE.MENTION'
    },
    [NotificationType.Assigned]: {
        icon: 'user-check',
        colorClass: 'notif-card-badge__assigned',
        labelKey: 'NOTIFICATION.TYPE.ASSIGNED'
    },
    [NotificationType.StateChanged]: {
        icon: 'circle-dot',
        colorClass: 'notif-card-badge__state',
        labelKey: 'NOTIFICATION.TYPE.STATE_CHANGED'
    },
    [NotificationType.SeverityEscalated]: {
        icon: 'flag',
        colorClass: 'notif-card-badge__severity',
        labelKey: 'NOTIFICATION.TYPE.SEVERITY_ESCALATED'
    },
    [NotificationType.SeverityDeescalated]: {
        icon: 'flag',
        colorClass: 'notif-card-badge__severity',
        labelKey: 'NOTIFICATION.TYPE.SEVERITY_DEESCALATED'
    },
    [NotificationType.TeamJoined]: {
        icon: 'user-plus',
        colorClass: 'notif-card-badge__team',
        labelKey: 'NOTIFICATION.TYPE.TEAM_JOINED'
    },
    [NotificationType.QualityScored]: {
        icon: 'star',
        colorClass: 'notif-card-badge__quality',
        labelKey: 'NOTIFICATION.TYPE.QUALITY_SCORED'
    }
};

@Component({
    selector: 'app-notification-card',
    templateUrl: './notification-card.component.html',
    styleUrls: ['./notification-card.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class NotificationCardComponent {
    public notification = input.required<Notification>();

    public dismiss = output<void>();

    public read = output<void>();

    protected readonly NotificationType = NotificationType;

    protected readonly chip = computed<TypeChip>(() => TYPE_CHIPS[this.notification().type]);

    protected readonly relativeTime = computed<string>(() =>
        formatDistanceToNow(new Date(this.notification().createdAt), {
            addSuffix: true
        })
    );

    protected readonly actionTextKey = computed<string | null>(() => {
        const map: Partial<Record<NotificationType, string>> = {
            [NotificationType.StateChanged]: 'NOTIFICATION.TEXT.STATE_CHANGED',
            [NotificationType.SeverityEscalated]: 'NOTIFICATION.TEXT.SEVERITY_ESCALATED',
            [NotificationType.SeverityDeescalated]: 'NOTIFICATION.TEXT.SEVERITY_DEESCALATED',
            [NotificationType.Assigned]: 'NOTIFICATION.TEXT.ASSIGNED'
        };
        return map[this.notification().type] ?? null;
    });

    protected readonly refAnchorRouterLink = computed<unknown[] | null>(() => {
        const { refPublicId, idProject, type } = this.notification();
        if (!refPublicId || !idProject) return null;
        if (
            type !== NotificationType.StateChanged &&
            type !== NotificationType.SeverityEscalated &&
            type !== NotificationType.SeverityDeescalated &&
            type !== NotificationType.Assigned &&
            type !== NotificationType.Comment &&
            type !== NotificationType.Mention &&
            type !== NotificationType.QualityScored
        )
            return null;
        return ['/project', idProject, 'issue', refPublicId];
    });

    protected readonly refAnchorText = computed<string | null>(() => {
        const { refPublicId, refTitle } = this.notification();
        if (!refPublicId) return null;
        return refTitle ? `#${refPublicId} ${refTitle}` : `#${refPublicId}`;
    });

    protected readonly stateBody = computed<NotificationBodyState | null>(() => {
        if (this.notification().type !== NotificationType.StateChanged) return null;
        return (this.notification().body as NotificationBodyState) ?? null;
    });

    protected readonly severityBody = computed<NotificationBodySeverity | null>(() => {
        if (
            this.notification().type !== NotificationType.SeverityEscalated &&
            this.notification().type !== NotificationType.SeverityDeescalated
        )
            return null;
        return (this.notification().body as NotificationBodySeverity) ?? null;
    });

    protected onDismiss(): void {
        this.dismiss.emit();
    }

    protected onMarkRead(): void {
        if (!this.notification().isRead) {
            this.read.emit();
        }
    }
}
