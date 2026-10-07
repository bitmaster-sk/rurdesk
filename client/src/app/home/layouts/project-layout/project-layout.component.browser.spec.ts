import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, RouterLink, RouterLinkActive, RouterOutlet, provideRouter } from '@angular/router';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { ProjectStore } from 'src/app/project/project.store';
import { TablerIconStub } from 'src/testing/stubs';
import { ProjectLayoutComponent } from './project-layout.component';

@Component({ selector: 'app-top-menu', template: '', standalone: true })
class TopMenuStub {}

@Component({ selector: 'app-page-stub', template: '', standalone: true })
class PageStub {}

class EmptyLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, string>> {
        return of({});
    }
}

async function render(url: string): Promise<HTMLElement> {
    await TestBed.configureTestingModule({
        declarations: [ProjectLayoutComponent],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            RouterOutlet,
            RouterLink,
            RouterLinkActive,
            TopMenuStub,
            TablerIconStub
        ],
        providers: [
            provideRouter([
                {
                    path: 'wiki',
                    data: { collapsedMenu: true },
                    children: [{ path: '', component: PageStub }]
                },
                { path: 'table', component: PageStub }
            ]),
            { provide: ProjectStore, useValue: { project$: of({ idProject: 1, name: 'Project' }) } }
        ]
    }).compileComponents();
    await TestBed.inject(Router).navigateByUrl(url);
    const fixture = TestBed.createComponent(ProjectLayoutComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
}

describe('ProjectLayoutComponent', () => {
    it('collapses the main menu to icons on a route that asks for it', async () => {
        const element = await render('/wiki');
        const menu = element.querySelector('[data-testid="project-left-menu"]');
        expect(menu?.classList).toContain('home-left-menu--collapsed');
        expect(
            element.querySelector('[data-testid="sidebar-link-wiki"]')?.getAttribute('href')
        ).toBe('/project/1/wiki');
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
