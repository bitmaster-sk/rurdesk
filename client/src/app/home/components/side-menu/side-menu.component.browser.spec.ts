import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, signal, viewChild } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, RouterLink, RouterLinkActive, RouterOutlet, provideRouter } from '@angular/router';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { AclStore } from 'src/app/project/store/acl.store';
import { UiMenuComponent } from 'src/app/ui/components/menu/menu.component';
import { TablerIconStub } from 'src/testing/stubs';
import { ProjectContextStore } from '../../store/project-context.store';
import { SideMenuComponent } from './side-menu.component';
// The spacing tokens the row and overlay geometry depend on live in the global styles.
import 'src/styles.scss';

@Component({ selector: 'app-page-stub', template: '', standalone: true })
class PageStub {}

@Component({ selector: 'app-project-switcher', template: '', standalone: false })
class ProjectSwitcherStub {}

const openPalette = vi.fn();
const toggleNotifications = vi.fn();

@Component({ selector: 'app-notification-menu', template: '', standalone: false })
class NotificationMenuStub {
    public readonly badge = signal('');
    public readonly isOpen = signal(false);
    public toggle(origin: HTMLElement): void {
        toggleNotifications(origin);
    }
}

// A real ui-menu, so the row's focus handling is exercised end to end.
@Component({
    selector: 'app-message-menu',
    template: '<ui-menu #menu placement="right" [model]="items" />',
    standalone: false
})
class MessageMenuStub {
    private readonly menu = viewChild<UiMenuComponent>('menu');
    public readonly items = [{ label: 'Team A', routerLink: ['/message', 1, 'team', 'view'] }];
    public readonly badge = signal('12');
    public readonly isOpen = computed(() => this.menu()?.isOpen() ?? false);
    public toggle(origin: HTMLElement): void {
        this.menu()?.toggle(origin);
    }
}

@Component({ selector: 'app-agent-chat-menu', template: '', standalone: false })
class AgentChatMenuStub {
    public readonly badge = signal('');
    public readonly isOpen = signal(false);
    public toggle(): void {}
}

// The global rows get their real labels so they are as wide as in the app.
class RowLabelLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, Record<string, string>>> {
        return of({
            NOTIFICATION: { TITLE: 'Notifications' },
            HOME: { MESSAGES: 'Messages' },
            AGENT_CHAT: { MENU_LABEL: 'Agent chat' }
        });
    }
}

async function render(
    url: string,
    { idProject = 1, isOwner = false }: { idProject?: number | null; isOwner?: boolean } = {}
): Promise<HTMLElement> {
    await TestBed.configureTestingModule({
        declarations: [
            SideMenuComponent,
            ProjectSwitcherStub,
            NotificationMenuStub,
            MessageMenuStub,
            AgentChatMenuStub,
            UiMenuComponent
        ],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: RowLabelLoader },
                fallbackLang: 'en'
            }),
            RouterOutlet,
            RouterLink,
            RouterLinkActive,
            NgTemplateOutlet,
            TablerIconStub
        ],
        providers: [
            provideRouter([
                {
                    path: 'wiki',
                    data: { collapsedMenu: true },
                    children: [{ path: '', component: PageStub }]
                },
                { path: '**', component: PageStub }
            ]),
            { provide: ProjectContextStore, useValue: { idProject: signal(idProject) } },
            { provide: AclStore, useValue: { canUpdateProject: signal(isOwner) } },
            { provide: CommandPaletteService, useValue: { open: openPalette } }
        ]
    }).compileComponents();
    await TestBed.inject(Router).navigateByUrl(url);
    const fixture = TestBed.createComponent(SideMenuComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
}

const href = (element: HTMLElement, testId: string): string | null | undefined =>
    element.querySelector(`[data-testid="${testId}"]`)?.getAttribute('href');

describe('SideMenuComponent', () => {
    it('links to the context project on a page outside the project', async () => {
        const element = await render('/message/1/team/view', { idProject: 4 });

        expect(href(element, 'sidebar-link-overview')).toBe('/project/4/view');
        expect(href(element, 'sidebar-link-table')).toBe('/project/4/issue/view/table');
    });

    it('opens the command palette from the search button in the header', async () => {
        const element = await render('/table');

        element.querySelector<HTMLElement>('[data-testid="side-menu-search"]')?.click();

        expect(openPalette).toHaveBeenCalled();
    });

    it('shows project settings only to the project owner', async () => {
        const member = await render('/table');
        expect(member.textContent).not.toContain('PROJECT.SETTINGS');
        TestBed.resetTestingModule();

        const owner = await render('/table', { isOwner: true });
        expect(owner.textContent).toContain('PROJECT.SETTINGS');
    });

    it('keeps only the global rows when the user has no project', async () => {
        const element = await render('/user', { idProject: null });

        expect(element.querySelector('[data-testid="project-switcher"]')).toBeNull();
        expect(element.querySelector('[data-testid="sidebar-link-overview"]')).toBeNull();
        expect(element.querySelector('[data-testid="sidebar-messages"]')).not.toBeNull();
    });

    it('shows the unread count on the row and opens the notification center', async () => {
        const element = await render('/table');
        const messages = element.querySelector('[data-testid="sidebar-messages"]');
        expect(messages?.querySelector('.count')?.textContent?.trim()).toBe('12');
        expect(element.querySelector('[data-testid="sidebar-notifications"] .count')).toBeNull();

        element.querySelector<HTMLElement>('[data-testid="sidebar-notifications"]')?.click();

        expect(toggleNotifications).toHaveBeenCalled();
    });

    it('marks Messages or Agent chat as the current page on their routes', async () => {
        const current = (element: HTMLElement): string[] =>
            ['sidebar-messages', 'agent-chat-menu'].filter(
                id =>
                    element.querySelector(`[data-testid="${id}"]`)?.getAttribute('aria-current') ===
                    'page'
            );

        expect(current(await render('/message/1/team/view'))).toEqual(['sidebar-messages']);
        TestBed.resetTestingModule();
        expect(current(await render('/agent-chat/5'))).toEqual(['agent-chat-menu']);
        TestBed.resetTestingModule();
        expect(current(await render('/table'))).toEqual([]);
    });

    it('opens the chats clear of the menu edge and returns focus to the row on Escape', async () => {
        const element = await render('/table');
        const menu = element.querySelector<HTMLElement>('[data-testid="project-left-menu"]')!;
        const row = element.querySelector<HTMLButtonElement>('[data-testid="sidebar-messages"]')!;
        row.focus();

        row.click();
        await vi.waitFor(() => expect(row.getAttribute('aria-expanded')).toBe('true'));
        const panel = document.querySelector<HTMLElement>('.ui-menu')!;
        expect(panel.getBoundingClientRect().left).toBeGreaterThanOrEqual(
            menu.getBoundingClientRect().right + 4
        );
        await vi.waitFor(() => expect(document.activeElement?.textContent).toContain('Team A'));

        document.activeElement?.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
        );

        expect(document.activeElement).toBe(row);
        await vi.waitFor(() => expect(row.getAttribute('aria-expanded')).toBe('false'));
    });

    it('collapses the menu to icons on a route that asks for it', async () => {
        const element = await render('/wiki');
        const menu = element.querySelector('[data-testid="project-left-menu"]');
        expect(menu?.classList).toContain('home-left-menu--collapsed');
        expect(href(element, 'sidebar-link-wiki')).toBe('/project/1/wiki');
    });

    it('keeps the unread count whole on the icon in the collapsed menu', async () => {
        const element = await render('/wiki');
        const row = element.querySelector<HTMLElement>('[data-testid="sidebar-messages"]')!;
        const count = row.querySelector<HTMLElement>('.count')!.getBoundingClientRect();
        const rail = element.querySelector<HTMLElement>('.menu-panel')!.getBoundingClientRect();

        expect(count.right).toBeLessThanOrEqual(row.getBoundingClientRect().right);
        expect(count.right).toBeLessThanOrEqual(rail.right);
    });

    it('expands the collapsed menu over the content while it has focus', async () => {
        const element = await render('/wiki');
        const panel = element.querySelector<HTMLElement>('.menu-panel');
        const label = element.querySelector<HTMLElement>('[data-testid="sidebar-link-wiki"] .text');
        const railWidth = panel!.getBoundingClientRect().width;
        expect(railWidth).toBeLessThan(60);
        expect(getComputedStyle(label!).opacity).toBe('0');

        element.querySelector<HTMLElement>('[data-testid="sidebar-link-wiki"]')?.focus();

        await vi.waitFor(() => {
            expect(panel?.getBoundingClientRect().width).toBeGreaterThan(3 * railWidth);
            expect(getComputedStyle(label!).opacity).toBe('1');
        });
    });

    it('stays closed after a click until the pointer leaves the menu', async () => {
        const element = await render('/wiki');
        const menu = element.querySelector<HTMLElement>('[data-testid="project-left-menu"]')!;

        menu.querySelector<HTMLElement>('.menu-section-title')?.click();
        await vi.waitFor(() => expect(menu.classList).toContain('home-left-menu--resting'));

        menu.dispatchEvent(new MouseEvent('mouseleave'));
        await vi.waitFor(() => expect(menu.classList).not.toContain('home-left-menu--resting'));
    });

    it('keeps the full menu everywhere else', async () => {
        const element = await render('/table');
        const menu = element.querySelector('[data-testid="project-left-menu"]');
        expect(menu?.classList).not.toContain('home-left-menu--collapsed');
    });
});
