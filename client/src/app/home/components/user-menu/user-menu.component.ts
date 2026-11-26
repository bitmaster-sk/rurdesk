import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    computed,
    inject,
    input
} from '@angular/core';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';

/** The signed-in user at the bottom of the side menu, with the account and admin links. */
@Component({
    selector: 'app-user-menu',
    templateUrl: './user-menu.component.html',
    styleUrls: ['./user-menu.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    host: { '[class.user-menu--rail]': 'isRail()' }
})
export class UserMenuComponent {
    /** In the folded side menu only the avatar shows; the tooltip names the user. */
    public readonly isRail = input(false);

    private readonly authStore = inject(AuthStore);
    private readonly palette = inject(CommandPaletteService);

    /** The menu anchors to the whole row, so it opens beside the side menu edge. */
    protected readonly host = inject<ElementRef<HTMLElement>>(ElementRef).nativeElement;

    protected readonly user = this.authStore.user;

    protected readonly items = computed<UiMenuItem[]>(() => {
        const items: UiMenuItem[] = [
            { labelKey: 'USER.MY_PAGE', icon: 'user', routerLink: ['/user'] },
            { labelKey: 'USER.SETTINGS', icon: 'settings', routerLink: ['/user/settings'] },
            {
                labelKey: 'COMMAND.ACTION.SHORTCUTS',
                icon: 'keyboard',
                command: () => this.palette.openHelp()
            }
        ];
        if (this.user()?.isAdmin) {
            items.push(
                { separator: true },
                {
                    labelKey: 'ADMIN.TITLE',
                    items: [
                        { labelKey: 'ADMIN.USERS', icon: 'users', routerLink: ['/admin/users'] },
                        {
                            labelKey: 'SKILL.TITLE',
                            icon: 'sparkles',
                            routerLink: ['/admin/skills']
                        },
                        {
                            labelKey: 'ADMIN.SETTINGS',
                            icon: 'shield',
                            routerLink: ['/admin/settings']
                        }
                    ]
                }
            );
        }
        items.push(
            { separator: true },
            { labelKey: 'LOGOUT', icon: 'logout', routerLink: ['/logout'] }
        );
        return items;
    });
}
