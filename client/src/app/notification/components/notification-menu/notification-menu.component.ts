import { ChangeDetectionStrategy, Component, computed, inject, viewChild } from '@angular/core';
import { UiPopoverComponent } from '../../../ui/components/popover/popover.component';
import { NotificationStore } from '../../store/notification.store';

/** Notification center behind the side menu row; the row itself lives in the side menu. */
@Component({
    selector: 'app-notification-menu',
    templateUrl: './notification-menu.component.html',
    styleUrls: ['./notification-menu.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class NotificationMenuComponent {
    private readonly store = inject(NotificationStore);

    // Not required: the side menu reads isOpen before this component's view query resolves.
    private readonly overlay = viewChild<UiPopoverComponent>('overlay');

    public readonly isOpen = computed(() => this.overlay()?.isOpen() ?? false);

    public readonly badge = computed<string>(() => {
        const count = this.store.unreadCount();
        return count > 99 ? '99+' : count > 0 ? String(count) : '';
    });

    public toggle(origin: HTMLElement): void {
        this.overlay()?.toggle(origin);
    }
}
