import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { UiModule } from '../../ui.module';

@Component({
    standalone: false,
    template: `
        <button #btn (click)="pop.toggle($event)">open</button>
        <ui-popover
            #pop
            [panelClass]="panelClass"
            [dismissable]="dismissable"
            (closed)="hideCount = hideCount + 1"
        >
            <div class="pop-content">Hello</div>
        </ui-popover>
    `
})
class HostComponent {
    public panelClass = '';
    public dismissable = true;
    public hideCount = 0;
}

describe('UiPopoverComponent (browser)', () => {
    function panel(): HTMLElement | null {
        return document.querySelector('.ui-popover');
    }
    function openButton(el: HTMLElement): HTMLElement {
        return el.querySelector('button') as HTMLElement;
    }
    function clickOutside(): void {
        document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
        document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule],
            providers: [provideNoopAnimations()]
        }).compileComponents();
    });

    function setup(configure?: (host: HostComponent) => void) {
        const fixture = TestBed.createComponent(HostComponent);
        configure?.(fixture.componentInstance);
        fixture.detectChanges();
        return fixture;
    }

    it('is closed initially', () => {
        setup();
        expect(panel()).toBeNull();
    });

    it('opens on toggle and projects its content into the overlay', () => {
        const fixture = setup();
        openButton(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(panel()).not.toBeNull();
        expect(panel()!.querySelector('.pop-content')?.textContent).toContain('Hello');
    });

    it('toggles closed on a second toggle and emits onHide', () => {
        const fixture = setup();
        const btn = openButton(fixture.nativeElement);
        btn.click();
        fixture.detectChanges();
        btn.click();
        fixture.detectChanges();
        expect(panel()).toBeNull();
        expect(fixture.componentInstance.hideCount).toBe(1);
    });

    it('dismisses on an outside click and emits onHide', () => {
        const fixture = setup();
        openButton(fixture.nativeElement).click();
        fixture.detectChanges();
        clickOutside();
        fixture.detectChanges();
        expect(panel()).toBeNull();
        expect(fixture.componentInstance.hideCount).toBe(1);
    });

    it('dismisses on Escape', () => {
        const fixture = setup();
        openButton(fixture.nativeElement).click();
        fixture.detectChanges();
        document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        fixture.detectChanges();
        expect(panel()).toBeNull();
    });

    it('stays open on outside click when not dismissable', () => {
        const fixture = setup(host => (host.dismissable = false));
        openButton(fixture.nativeElement).click();
        fixture.detectChanges();
        clickOutside();
        fixture.detectChanges();
        expect(panel()).not.toBeNull();
        expect(fixture.componentInstance.hideCount).toBe(0);
    });

    it('applies panelClass to the overlay surface', () => {
        const fixture = setup(host => (host.panelClass = 'my-panel'));
        openButton(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(document.querySelector('.ui-popover.my-panel')).not.toBeNull();
    });

    it('does not dismiss when clicking inside the panel', () => {
        const fixture = setup();
        openButton(fixture.nativeElement).click();
        fixture.detectChanges();
        const content = panel()!.querySelector('.pop-content') as HTMLElement;
        content.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
        content.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        fixture.detectChanges();
        expect(panel()).not.toBeNull();
        expect(fixture.componentInstance.hideCount).toBe(0);
    });
});
