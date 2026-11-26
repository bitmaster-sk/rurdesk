import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, RouterOutlet, provideRouter } from '@angular/router';
import { describe, expect, it, vi } from 'vitest';
import { ProjectContextStore } from '../../store/project-context.store';
import { ShellLayoutComponent } from './shell-layout.component';
// The shell lays out with Tailwind utilities.
import 'src/tailwind.css';

@Component({
    selector: 'app-page-stub',
    template: '<div data-testid="page">page</div>',
    standalone: true
})
class PageStub {}

@Component({ selector: 'app-license-banner', template: '', standalone: false })
class LicenseBannerStub {}

@Component({
    selector: 'app-side-menu',
    template: '<nav data-testid="side-menu" style="width: 12rem; height: 100%"></nav>',
    standalone: false
})
class SideMenuStub {}

describe('ShellLayoutComponent', () => {
    it('puts the side menu beside the page with no bar above them', async () => {
        const load = vi.fn();
        await TestBed.configureTestingModule({
            declarations: [ShellLayoutComponent, LicenseBannerStub, SideMenuStub],
            imports: [RouterOutlet],
            providers: [
                provideRouter([{ path: '**', component: PageStub }]),
                { provide: ProjectContextStore, useValue: { load } }
            ]
        }).compileComponents();
        const fixture = TestBed.createComponent(ShellLayoutComponent);
        fixture.autoDetectChanges();
        await TestBed.inject(Router).navigateByUrl('/table');
        await fixture.whenStable();

        const element = fixture.nativeElement as HTMLElement;
        const shell = element.getBoundingClientRect();
        const menu = element.querySelector('[data-testid="side-menu"]')!.getBoundingClientRect();
        const page = element.querySelector('[data-testid="page"]')!.getBoundingClientRect();
        expect(load).toHaveBeenCalled();
        expect(menu.top).toBe(shell.top);
        expect(page.top).toBe(shell.top);
        expect(page.left).toBeGreaterThanOrEqual(menu.right);
    });
});
