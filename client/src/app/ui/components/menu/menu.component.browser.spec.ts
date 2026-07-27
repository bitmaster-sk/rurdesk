import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { provideRouter } from '@angular/router';
import { UiModule } from '../../ui.module';
import { UiMenuItem } from './menu-item.model';

interface HostMenuItem extends UiMenuItem {
    tablerIcon?: string;
}

@Component({
    standalone: false,
    template: `
        <button #trigger (click)="menu.toggle($event)">open</button>
        <ui-menu #menu [panelClass]="panelClass" [model]="model">
            @if (useTemplate) {
                <ng-template #item let-item>
                    <span class="custom-row">★ {{ item.label }}</span>
                </ng-template>
            }
        </ui-menu>
    `
})
class HostComponent {
    public panelClass = '';
    public useTemplate = false;
    public lastCommand = '';
    public model: HostMenuItem[] = [
        { label: 'Rename', command: () => (this.lastCommand = 'Rename') },
        { label: 'Delete', command: () => (this.lastCommand = 'Delete') }
    ];
}

describe('UiMenuComponent (browser)', () => {
    function panel(): HTMLElement | null {
        return document.querySelector('.ui-menu');
    }
    function items(): HTMLElement[] {
        return Array.from(document.querySelectorAll<HTMLElement>('.ui-menu [role="menuitem"]'));
    }
    function trigger(el: HTMLElement): HTMLElement {
        return el.querySelector('button') as HTMLElement;
    }
    function key(k: string): void {
        document.body.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }));
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule],
            providers: [provideNoopAnimations(), provideRouter([])]
        }).compileComponents();
    });

    async function open(configure?: (host: HostComponent) => void) {
        const fixture = TestBed.createComponent(HostComponent);
        configure?.(fixture.componentInstance);
        fixture.detectChanges();
        trigger(fixture.nativeElement).click();
        fixture.detectChanges();
        await Promise.resolve(); // flush queueMicrotask focus-first
        fixture.detectChanges();
        return fixture;
    }

    it('is closed initially', () => {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        expect(panel()).toBeNull();
    });

    it('opens on toggle and renders one menuitem per item', async () => {
        await open();
        expect(panel()).not.toBeNull();
        expect(items().length).toBe(2);
        expect(items()[0].tagName).toBe('BUTTON'); // command → <button>
    });

    it('toggles closed on a second toggle', async () => {
        const fixture = await open();
        trigger(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(panel()).toBeNull();
    });

    it('runs command and closes on click', async () => {
        const fixture = await open();
        items()[1].click();
        fixture.detectChanges();
        expect(fixture.componentInstance.lastCommand).toBe('Delete');
        expect(panel()).toBeNull();
    });

    it('renders routerLink items as <a role="menuitem">', async () => {
        await open(h => (h.model = [{ label: 'Home', routerLink: ['/'] }]));
        expect(items()[0].tagName).toBe('A');
        expect(items()[0].getAttribute('href')).toBe('/');
    });

    it('renders a group as role=group with aria-label; empty group is header-only', async () => {
        await open(
            h =>
                (h.model = [
                    { label: 'Projects', items: [{ label: 'Alpha', routerLink: ['/'] }] },
                    { label: 'Teams', items: [] }
                ])
        );
        const group = document.querySelector('.ui-menu [role="group"]');
        expect(group?.getAttribute('aria-label')).toBe('Projects');
        // two group headers rendered, but only the non-empty group has a menuitem
        expect(document.querySelectorAll('.ui-menu .ui-menu-group-label').length).toBe(2);
        expect(items().length).toBe(1);
    });

    it('renders separators and skips them as menuitems', async () => {
        await open(
            h =>
                (h.model = [
                    { label: 'A', command: () => {} },
                    { separator: true },
                    { label: 'B', command: () => {} }
                ])
        );
        expect(document.querySelectorAll('.ui-menu [role="separator"]').length).toBe(1);
        expect(items().length).toBe(2);
    });

    it('projects a custom #item template into the interior', async () => {
        await open(h => (h.useTemplate = true));
        expect(items()[0].querySelector('.custom-row')?.textContent).toContain('★ Rename');
    });

    it('dismisses on outside click and on Escape', async () => {
        const fixture = await open();
        document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
        document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        fixture.detectChanges();
        expect(panel()).toBeNull();

        const fixture2 = await open();
        key('Escape');
        fixture2.detectChanges();
        expect(panel()).toBeNull();
    });

    it('applies panelClass to the overlay pane', async () => {
        await open(h => (h.panelClass = 'user-menu'));
        expect(document.querySelector('.cdk-overlay-pane.user-menu')).not.toBeNull();
    });

    // D1 — restore focus to the trigger on Escape
    it('returns focus to the trigger on Escape', async () => {
        const fixture = await open();
        key('Escape');
        fixture.detectChanges();
        expect(document.activeElement).toBe(trigger(fixture.nativeElement));
    });

    // D5 — arrow navigation reaches every leaf across nested groups
    it('ArrowDown walks every menuitem across groups', async () => {
        await open(
            h =>
                (h.model = [
                    {
                        label: 'G1',
                        items: [
                            { label: 'a', command: () => {} },
                            { label: 'b', command: () => {} }
                        ]
                    },
                    { label: 'G2', items: [{ label: 'c', command: () => {} }] }
                ])
        );
        const rows = items();
        expect(rows.length).toBe(3);
        expect(document.activeElement).toBe(rows[0]); // focus-first
        key('ArrowDown');
        expect(document.activeElement).toBe(rows[1]);
        key('ArrowDown');
        expect(document.activeElement).toBe(rows[2]); // crossed from G1 into G2
        key('ArrowDown');
        expect(document.activeElement).toBe(rows[0]); // wraps
    });

    // D4 — Space activates the focused row (single path) and closes
    it('activates the focused command row on Space and closes', async () => {
        const fixture = await open();
        expect(document.activeElement).toBe(items()[0]);
        key(' ');
        fixture.detectChanges();
        expect(fixture.componentInstance.lastCommand).toBe('Rename');
        expect(panel()).toBeNull();
    });
});
