import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';

interface AppMenuItem extends UiMenuItem {
    tablerIcon?: string;
}
import { AuthStore } from 'src/app/auth/store/auth.store';

@Component({
    selector: 'app-top-menu',
    templateUrl: './top-menu.component.html',
    styleUrls: ['./top-menu.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class TopMenuComponent {
    private readonly i18n = inject(I18nService);
    private readonly authStore = inject(AuthStore);

    protected readonly user = this.authStore.user;

    protected readonly userMenuItems = computed<AppMenuItem[]>(() => {
        const items: AppMenuItem[] = [
            {
                label: this.i18n.instant('USER.MY_PAGE'),
                tablerIcon: 'user',
                routerLink: ['/user']
            },
            {
                label: this.i18n.instant('USER.SETTINGS'),
                tablerIcon: 'settings',
                routerLink: ['/user/settings']
            },
            {
                label: this.i18n.instant('LOGOUT'),
                tablerIcon: 'logout',
                routerLink: ['/logout']
            }
        ];
        if (this.user()?.isAdmin) {
            items.push(
                { separator: true },
                {
                    label: this.i18n.instant('ADMIN.USERS'),
                    tablerIcon: 'users',
                    routerLink: ['/admin/users']
                },
                {
                    label: this.i18n.instant('SKILL.TITLE'),
                    tablerIcon: 'sparkles',
                    routerLink: ['/admin/skills']
                },
                {
                    label: this.i18n.instant('ADMIN.SETTINGS'),
                    tablerIcon: 'settings',
                    routerLink: ['/admin/settings']
                }
            );
        }
        return items;
    });
}
