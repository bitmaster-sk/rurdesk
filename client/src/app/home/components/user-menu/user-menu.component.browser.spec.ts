import { NgTemplateOutlet } from '@angular/common';
import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { RouterLink, RouterLinkActive, provideRouter } from '@angular/router';
import { TranslateLoader, TranslateModule, TranslationObject } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { UiMenuComponent } from 'src/app/ui/components/menu/menu.component';
import { AvatarStub, TablerIconStub } from 'src/testing/stubs';
import { UserMenuComponent } from './user-menu.component';
// The spacing tokens the row and overlay geometry depend on live in the global styles.
import 'src/styles.scss';

const regular: User = { idUser: 1, name: 'Rita Regular', email: 'r@r.sk', colorAvatarBg: '#111' };
const admin: User = { ...regular, idUser: 2, name: 'Adam Admin', isAdmin: true };

class LabelLoader implements TranslateLoader {
    public getTranslation(): Observable<TranslationObject> {
        return of({
            USER: { MY_PAGE: 'My page', SETTINGS: 'User settings' },
            COMMAND: { ACTION: { SHORTCUTS: 'Keyboard shortcuts' } },
            ADMIN: {
                TITLE: 'Administration',
                USERS: 'Users',
                SETTINGS: 'Application settings',
                ADMIN: 'Admin',
                USER: 'User'
            },
            SKILL: { TITLE: 'Skills' },
            LOGOUT: 'Log out'
        });
    }
}

const openHelp = vi.fn();

async function render(user: User): Promise<HTMLElement> {
    await TestBed.configureTestingModule({
        declarations: [UserMenuComponent, UiMenuComponent],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: LabelLoader },
                fallbackLang: 'en'
            }),
            RouterLink,
            RouterLinkActive,
            NgTemplateOutlet,
            TablerIconStub,
            AvatarStub
        ],
        providers: [
            provideRouter([]),
            { provide: AuthStore, useValue: { user: signal(user) } },
            { provide: CommandPaletteService, useValue: { openHelp } }
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(UserMenuComponent);
    // As wide as the side menu it sits in, so the panel has room beside it.
    (fixture.nativeElement as HTMLElement).style.width = '12rem';
    fixture.autoDetectChanges();
    await fixture.whenStable();
    return fixture.nativeElement as HTMLElement;
}

const trigger = (element: HTMLElement): HTMLButtonElement =>
    element.querySelector<HTMLButtonElement>('[data-testid="avatar-trigger"]')!;

/** The whole visible text of each row, so nothing beyond the label slips in. */
const rows = (): string[] =>
    [...document.querySelectorAll('.ui-menu [role="menuitem"]')].map(
        row => row.textContent?.trim() ?? ''
    );

async function open(element: HTMLElement): Promise<void> {
    trigger(element).click();
    await vi.waitFor(() => expect(trigger(element).getAttribute('aria-expanded')).toBe('true'));
}

describe('UserMenuComponent', () => {
    it('shows who is signed in and their role', async () => {
        const element = await render(admin);

        expect(trigger(element).getAttribute('aria-label')).toBe('Adam Admin');
        expect(trigger(element).querySelector('.name')?.textContent?.trim()).toBe('Adam Admin');
        expect(trigger(element).querySelector('.role')?.textContent?.trim()).toBe('Admin');
    });

    it('lists the account links and logout, without administration, for a regular user', async () => {
        const element = await render(regular);
        expect(trigger(element).querySelector('.role')?.textContent?.trim()).toBe('User');

        await open(element);

        expect(rows()).toEqual(['My page', 'User settings', 'Keyboard shortcuts', 'Log out']);
        expect(document.querySelector('.ui-menu [role="group"]')).toBeNull();
    });

    it('adds the administration group before logout for an admin', async () => {
        const element = await render(admin);

        await open(element);

        const group = document.querySelector('.ui-menu [role="group"]');
        expect(group?.getAttribute('aria-label')).toBe('Administration');
        expect(group?.querySelectorAll('[role="menuitem"]')).toHaveLength(3);
        expect(rows()).toEqual([
            'My page',
            'User settings',
            'Keyboard shortcuts',
            'Users',
            'Skills',
            'Application settings',
            'Log out'
        ]);
        expect(document.querySelector('.ui-menu a[href="/logout"]')).not.toBeNull();
    });

    it('opens the keyboard shortcuts help and closes the menu', async () => {
        const element = await render(regular);
        await open(element);

        [...document.querySelectorAll<HTMLElement>('.ui-menu [role="menuitem"]')]
            .find(row => row.textContent?.includes('Keyboard shortcuts'))
            ?.click();

        expect(openHelp).toHaveBeenCalled();
        await vi.waitFor(() => expect(document.querySelector('.ui-menu')).toBeNull());
    });

    it('opens beside the row, not over it', async () => {
        const element = await render(regular);

        await open(element);

        const panel = document.querySelector<HTMLElement>('.ui-menu')!.getBoundingClientRect();
        expect(panel.left).toBeGreaterThanOrEqual(element.getBoundingClientRect().right);
    });
});
