import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, RouterLink, RouterLinkActive, RouterOutlet, provideRouter } from '@angular/router';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { AclStore } from 'src/app/project/store/acl.store';
import { TablerIconStub } from 'src/testing/stubs';
import { ProjectContextStore } from '../../store/project-context.store';
import { SideMenuComponent } from './side-menu.component';

@Component({ selector: 'app-page-stub', template: '', standalone: true })
class PageStub {}

@Component({ selector: 'app-project-switcher', template: '', standalone: false })
class ProjectSwitcherStub {}

const openPalette = vi.fn();

class EmptyLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, string>> {
        return of({});
    }
}

async function render(
    url: string,
    { idProject = 1, isOwner = false }: { idProject?: number | null; isOwner?: boolean } = {}
): Promise<HTMLElement> {
    await TestBed.configureTestingModule({
        declarations: [SideMenuComponent, ProjectSwitcherStub],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            RouterOutlet,
            RouterLink,
            RouterLinkActive,
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

    it('renders no menu when the user has no project', async () => {
        const element = await render('/user', { idProject: null });

        expect(element.querySelector('[data-testid="project-left-menu"]')).toBeNull();
    });

    it('collapses the menu to icons on a route that asks for it', async () => {
        const element = await render('/wiki');
        const menu = element.querySelector('[data-testid="project-left-menu"]');
        expect(menu?.classList).toContain('home-left-menu--collapsed');
        expect(href(element, 'sidebar-link-wiki')).toBe('/project/1/wiki');
    });

    it('expands the collapsed menu over the content while it has focus', async () => {
        const element = await render('/wiki');
        const panel = element.querySelector<HTMLElement>('.menu-panel');
        const label = element.querySelector<HTMLElement>('[data-testid="sidebar-link-wiki"] .text');
        expect(panel?.getBoundingClientRect().width).toBeLessThan(60);
        expect(getComputedStyle(label!).opacity).toBe('0');

        element.querySelector<HTMLElement>('[data-testid="sidebar-link-wiki"]')?.focus();

        await vi.waitFor(() => {
            expect(panel?.getBoundingClientRect().width).toBeGreaterThan(180);
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
