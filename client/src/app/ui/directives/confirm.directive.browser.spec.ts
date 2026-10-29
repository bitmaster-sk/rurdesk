import { ApplicationRef, Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../ui.module';

@Component({
    standalone: false,
    template: `
        <button uiConfirm [confirmText]="'Delete this?'" (confirmed)="onConfirmed()"></button>
    `
})
class HostComponent {
    public confirmedCount = 0;
    public onConfirmed(): void {
        this.confirmedCount++;
    }
}

@Component({
    standalone: false,
    template: `
        <button
            uiConfirm
            style="position: fixed; top: 0; right: 0; width: 6rem; height: 2rem"
            [confirmText]="'Publish this release to everyone?'"
        ></button>
    `
})
class CornerHostComponent {}

describe('UiConfirmDirective (browser)', () => {
    function panel(): HTMLElement | null {
        return document.querySelector('.ui-confirm-panel');
    }

    function buttons(): HTMLButtonElement[] {
        return Array.from(document.querySelectorAll('.ui-confirm-panel button'));
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent, CornerHostComponent],
            imports: [UiModule, TranslateModule.forRoot()],
            providers: [provideNoopAnimations()]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        const trigger = fixture.nativeElement.querySelector('button') as HTMLButtonElement;
        return { fixture, trigger };
    }

    it('opens the popup with the given text when the host is clicked', () => {
        const { trigger } = setup();
        expect(panel()).toBeNull();

        trigger.click();
        // The popup is its own component tree attached to ApplicationRef; tick to render its bindings.
        TestBed.inject(ApplicationRef).tick();

        expect(panel()).not.toBeNull();
        expect(panel()!.textContent).toContain('Delete this?');
    });

    it('emits confirmed and closes when accept is clicked', () => {
        const { fixture, trigger } = setup();
        trigger.click();

        // Template order: reject (No) first, accept (Yes) second.
        buttons()[1].click();
        fixture.detectChanges();

        expect(fixture.componentInstance.confirmedCount).toBe(1);
        expect(panel()).toBeNull();
    });

    it('closes without emitting when reject is clicked', () => {
        const { fixture, trigger } = setup();
        trigger.click();

        buttons()[0].click();
        fixture.detectChanges();

        expect(fixture.componentInstance.confirmedCount).toBe(0);
        expect(panel()).toBeNull();
    });

    it('closes on an outside pointer event without emitting', () => {
        const { fixture, trigger } = setup();
        trigger.click();
        expect(panel()).not.toBeNull();

        // CDK fires outsidePointerEvents on the click that follows a pointerdown.
        document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
        document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        fixture.detectChanges();

        expect(fixture.componentInstance.confirmedCount).toBe(0);
        expect(panel()).toBeNull();
    });

    it('keeps the popup below a button in the top right corner of the viewport', () => {
        const fixture = TestBed.createComponent(CornerHostComponent);
        fixture.detectChanges();
        const trigger = fixture.nativeElement.querySelector('button') as HTMLButtonElement;

        trigger.click();
        TestBed.inject(ApplicationRef).tick();

        const popup = panel()!.getBoundingClientRect();
        expect(popup.top).toBeGreaterThanOrEqual(trigger.getBoundingClientRect().bottom);
        expect(popup.right).toBeLessThanOrEqual(window.innerWidth);
    });
});
