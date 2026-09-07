import { Injector, runInInjectionContext, signal } from '@angular/core';
import { of } from 'rxjs';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { TopMenuComponent } from './top-menu.component';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { ProjectService } from 'src/app/project/project.service';
import { ProjectStore } from 'src/app/project/project.store';
import { WindowService } from 'src/app/shared/window/window.service';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { Router } from '@angular/router';
import { User } from 'src/app/auth/model/user.model';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';

const regular: User = {
    idUser: 1,
    name: 'Reg',
    email: 'r@r.com',
    colorAvatarBg: '#111'
};
const admin: User = { ...regular, idUser: 2, name: 'Adm', isAdmin: true };

function setup(user: User) {
    const injector = Injector.create({
        providers: [
            // instant() echoes the key so assertions stay i18n-independent
            { provide: I18nService, useValue: { instant: (k: string) => k } },
            { provide: AuthStore, useValue: { user: signal(user) } },
            { provide: ProjectService, useValue: { loadProjects: () => of([]) } },
            { provide: ProjectStore, useValue: { project$: of(null) } },
            { provide: WindowService, useValue: { open: () => ({ onClose: of() }) } },
            { provide: TrackerService, useValue: { isTracking$: of(false) } },
            { provide: Router, useValue: { url: '/', events: of() } }
        ]
    });
    const component = runInInjectionContext(injector, () => new TopMenuComponent());
    // userMenuItems is a protected computed signal
    return (component as unknown as { userMenuItems: () => UiMenuItem[] }).userMenuItems();
}

function links(items: UiMenuItem[]): string[] {
    return items.map(i => (i.routerLink as string[] | undefined)?.join('/') ?? '');
}

describe('TopMenuComponent — userMenuItems', () => {
    it('non-admin: only my-page, user settings, logout — no admin group', () => {
        const items = setup(regular);

        expect(items).toHaveLength(3);
        expect(items.some(i => i.items)).toBe(false);
        expect(links(items)).toEqual(['/user', '/user/settings', '/logout']);
    });

    it('admin: appends a separator then the admin links after logout', () => {
        const items = setup(admin);

        // base 3 links unchanged and first
        expect(links(items.slice(0, 3))).toEqual(['/user', '/user/settings', '/logout']);

        // followed by a separator
        expect(items[3].separator).toBe(true);

        // then the admin links
        expect(links(items.slice(4))).toEqual(['/admin/users', '/admin/skills', '/admin/settings']);
    });
});
