import { ChangeDetectionStrategy, Component, inject, viewChild } from '@angular/core';
import { UiPopoverComponent } from '../../../ui/components/popover/popover.component';
import { NotificationStore } from '../../store/notification.store';

@Component({
    selector: 'app-notification-menu',
    templateUrl: './notification-menu.component.html',
    styleUrls: ['./notification-menu.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class NotificationMenuComponent {
    private store = inject(NotificationStore);

    private overlay = viewChild.required<UiPopoverComponent>('overlay');

    protected unreadCount = this.store.unreadCount;

    protected onBellClick(event: MouseEvent): void {
        this.overlay().toggle(event);
    }
}
