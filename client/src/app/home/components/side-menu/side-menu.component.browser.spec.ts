import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, input, signal, viewChild } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, RouterLink, RouterLinkActive, RouterOutlet, provideRouter } from '@angular/router';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { userEvent } from 'vitest/browser';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { HotkeyService, RailToggleHandler } from 'src/app/core/command/hotkey.service';
import { TrackerControlVariant } from 'src/app/shared/tracker/constants/tracker-control-variant.enum';
import { UiMenuComponent } from 'src/app/ui/components/menu/menu.component';
import { UiTooltipComponent } from 'src/app/ui/components/tooltip/tooltip.component';
import { UiTooltipDirective } from 'src/app/ui/directives/tooltip.directive';
import { TablerIconStub } from 'src/testing/stubs';
import { ProjectContextStore } from '../../store/project-context.store';
import { SideMenuComponent } from './side-menu.component';
// The spacing tokens the row and overlay geometry depend on live in the global styles.
import 'src/styles.scss';

@Component({ selector: 'app-page-stub', template: '', standalone: true })
class PageStub {}

@Component({ selector: 'app-project-switcher', template: '', standalone: false })
class ProjectSwitcherStub {
    public readonly isRail = input(false);
}

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

@Component({
    selector: 'app-tracker-control',
    template: '<div data-testid="tracker-control" style="height: 2rem"></div>',
    standalone: false
})
class TrackerControlStub {
    public readonly variant = input<TrackerControlVariant>(TrackerControlVariant.Bar);
}

@Component({
    selector: 'app-user-menu',
    template: '<button data-testid="avatar-trigger">Me</button>',
    standalone: false
})
class UserMenuStub {
    public readonly isRail = input(false);
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

const COLLAPSED_KEY = 'sideMenuCollapsed';

let railToggle: RailToggleHandler | null = null;

async function render(
    url: string,
    {
        idProject = 1,
        isOwner = false,
        isCollapsed = false
    }: { idProject?: number | null; isOwner?: boolean; isCollapsed?: boolean } = {}
): Promise<HTMLElement> {
    // Saved by an earlier visit, as the store reads it on load.
    localStorage.setItem(COLLAPSED_KEY, isCollapsed ? '1' : '0');
    await TestBed.configureTestingModule({
        declarations: [
            SideMenuComponent,
            ProjectSwitcherStub,
            NotificationMenuStub,
            MessageMenuStub,
            AgentChatMenuStub,
            TrackerControlStub,
            UserMenuStub,
            UiMenuComponent,
            UiTooltipDirective,
            UiTooltipComponent
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
            provideRouter([{ path: '**', component: PageStub }]),
            {
                provide: ProjectContextStore,
                useValue: { idProject: signal(idProject), canUpdateProject: signal(isOwner) }
            },
            { provide: CommandPaletteService, useValue: { open: openPalette } },
            {
                provide: HotkeyService,
                useValue: {
                    registerRailToggle: (handler: RailToggleHandler | null) =>
                        (railToggle = handler)
                }
            }
        ]
    }).compileComponents();
    await TestBed.inject(Router).navigateByUrl(url);
    const fixture = TestBed.createComponent(SideMenuComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
}

const menuOf = (element: HTMLElement): HTMLElement =>
    element.querySelector<HTMLElement>('[data-testid="project-left-menu"]')!;

const tooltip = (): string | undefined =>
    document.querySelector('.ui-tooltip')?.textContent?.trim();

const href = (element: HTMLElement, testId: string): string | null | undefined =>
    element.querySelector(`[data-testid="${testId}"]`)?.getAttribute('href');

describe('SideMenuComponent', () => {
    afterEach(() => localStorage.removeItem(COLLAPSED_KEY));

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

    // Regression: the shortcut hint grew the search button on hover and pushed the fold button
    // past the menu edge.
    it('keeps the fold button clear of the menu edge while the search names its shortcut', async () => {
        const tokens = document.createElement('style');
        tokens.textContent =
            ':root { --ui-space-s: 0.25rem; --ui-space: 0.5rem; --ui-space-m: 0.75rem; --ui-space-l: 1rem; }' +
            ' tabler-icon { display: inline-block; width: 16px; height: 16px; }';
        document.head.append(tokens);
        try {
            const element = await render('/table');
            const search = element.querySelector<HTMLElement>('[data-testid="side-menu-search"]')!;
            const toggle = element.querySelector<HTMLElement>('[data-testid="side-menu-toggle"]')!;
            const inside = (): number =>
                menuOf(element).getBoundingClientRect().left +
                menuOf(element).clientWidth -
                toggle.getBoundingClientRect().right;

            const before = toggle.getBoundingClientRect().left;
            expect(inside()).toBeGreaterThanOrEqual(8);

            await userEvent.hover(search);

            await vi.waitFor(() => expect(tooltip()).toBe('HOME.SEARCH (⌘K)'));
            expect(toggle.getBoundingClientRect().left).toBe(before);
            expect(inside()).toBeGreaterThanOrEqual(8);
            // The tip must not cover the fold button right next to the search.
            const tip = document.querySelector('.ui-tooltip')!.getBoundingClientRect();
            expect(tip.top).toBeGreaterThanOrEqual(toggle.getBoundingClientRect().bottom);
            // A pointer left here would hover the next test's search button.
            await userEvent.unhover(search);
        } finally {
            tokens.remove();
        }
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

    it('keeps the timer above the global part and the user at the very bottom', async () => {
        const element = await render('/table');
        const box = (testId: string): DOMRect =>
            element.querySelector(`[data-testid="${testId}"]`)!.getBoundingClientRect();
        const menu = box('project-left-menu');

        expect(box('tracker-control').bottom).toBeLessThanOrEqual(box('sidebar-notifications').top);
        expect(box('agent-chat-menu').bottom).toBeLessThanOrEqual(box('avatar-trigger').top);
        expect(menu.bottom - box('avatar-trigger').bottom).toBeLessThan(24);
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

    it('shows the full menu until the user folds it, on the wiki too', async () => {
        const element = await render('/project/1/wiki');

        expect(menuOf(element).classList).not.toContain('home-left-menu--collapsed');
        expect(menuOf(element).getBoundingClientRect().width).toBeGreaterThan(150);
    });

    it('folds to an icon rail from the header button and opens again', async () => {
        const element = await render('/table');
        const toggle = element.querySelector<HTMLElement>('[data-testid="side-menu-toggle"]')!;

        toggle.click();

        await vi.waitFor(() =>
            expect(menuOf(element).classList).toContain('home-left-menu--collapsed')
        );
        expect(menuOf(element).getBoundingClientRect().width).toBeLessThan(60);
        expect(toggle.getAttribute('aria-label')).toBe('HOME.EXPAND_MENU');

        toggle.click();

        await vi.waitFor(() =>
            expect(menuOf(element).classList).not.toContain('home-left-menu--collapsed')
        );
        expect(toggle.getAttribute('aria-label')).toBe('HOME.COLLAPSE_MENU');
    });

    // Regression: the tip stayed where the fold button had been before the menu changed width.
    it('keeps the tip of the fold button on the button as the menu folds and opens', async () => {
        const element = await render('/table');
        const toggle = element.querySelector<HTMLElement>('[data-testid="side-menu-toggle"]')!;
        const tip = (): DOMRect => document.querySelector('.ui-tooltip')!.getBoundingClientRect();
        toggle.focus();
        await vi.waitFor(() => expect(tooltip()).toBe('HOME.COLLAPSE_MENU ([)'));
        expect(tip().top).toBeGreaterThanOrEqual(toggle.getBoundingClientRect().bottom);

        toggle.click();

        await vi.waitFor(() => {
            expect(tooltip()).toBe('HOME.EXPAND_MENU ([)');
            expect(tip().left).toBeGreaterThanOrEqual(toggle.getBoundingClientRect().right);
        });

        toggle.click();

        await vi.waitFor(() => {
            expect(tooltip()).toBe('HOME.COLLAPSE_MENU ([)');
            expect(tip().top).toBeGreaterThanOrEqual(toggle.getBoundingClientRect().bottom);
        });
    });

    it('folds from the [ shortcut', async () => {
        const element = await render('/table');

        railToggle?.toggle();

        await vi.waitFor(() =>
            expect(menuOf(element).classList).toContain('home-left-menu--collapsed')
        );
    });

    it('opens in the rail when the user left it folded', async () => {
        const element = await render('/table', { isCollapsed: true });

        expect(menuOf(element).classList).toContain('home-left-menu--collapsed');
    });

    it('names every rail item and shows the name on focus', async () => {
        const element = await render('/table', { isCollapsed: true });
        const controls = [...menuOf(element).querySelectorAll<HTMLElement>('a, button')];

        for (const control of controls) {
            const name = control.getAttribute('aria-label') ?? control.textContent?.trim();
            expect(name, control.outerHTML).toBeTruthy();
        }
        for (const label of menuOf(element).querySelectorAll('.link .text')) {
            expect(label.getBoundingClientRect().width).toBeLessThanOrEqual(1);
        }

        element.querySelector<HTMLElement>('[data-testid="sidebar-messages"]')!.focus();
        await vi.waitFor(() => expect(tooltip()).toBe('Messages'));

        element.querySelector<HTMLElement>('[data-testid="side-menu-toggle"]')!.focus();
        await vi.waitFor(() => expect(tooltip()).toBe('HOME.EXPAND_MENU ([)'));
    });

    it('keeps the row tooltips out of the full menu', async () => {
        const element = await render('/table');

        element.querySelector<HTMLElement>('[data-testid="sidebar-messages"]')!.focus();
        await new Promise(resolve => setTimeout(resolve, 300));

        expect(tooltip()).toBeUndefined();
    });

    it('lines every rail icon up on one vertical axis', async () => {
        // The icon stub has no box of its own; give it the real icon's size.
        const size = document.createElement('style');
        size.textContent = 'tabler-icon { display: inline-block; width: 16px; height: 16px; }';
        document.head.append(size);
        try {
            const element = await render('/table', { isCollapsed: true, isOwner: true });
            // The axis of the rail's inside, without its right border.
            const axis =
                menuOf(element).getBoundingClientRect().left + menuOf(element).clientWidth / 2;
            const icons = [
                ...menuOf(element).querySelectorAll('.brand svg, .brand tabler-icon, .link .icon')
            ];

            expect(icons.length).toBeGreaterThan(10);
            for (const icon of icons) {
                const box = icon.getBoundingClientRect();
                expect(box.left + box.width / 2, icon.outerHTML).toBeCloseTo(axis, 0);
            }
        } finally {
            size.remove();
        }
    });

    it('keeps the unread count whole on the icon in the rail', async () => {
        const element = await render('/table', { isCollapsed: true });
        const row = element.querySelector<HTMLElement>('[data-testid="sidebar-messages"]')!;
        const count = row.querySelector<HTMLElement>('.count')!.getBoundingClientRect();

        expect(count.right).toBeLessThanOrEqual(row.getBoundingClientRect().right);
        expect(count.right).toBeLessThanOrEqual(menuOf(element).getBoundingClientRect().right);
    });
});
