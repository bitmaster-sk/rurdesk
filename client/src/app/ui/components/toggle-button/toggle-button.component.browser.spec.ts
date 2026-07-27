import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { UiModule } from '../../ui.module';

@Component({
    standalone: false,
    template: `
        <ui-toggle-button [formControl]="ctrl">
            <span class="label">Filter</span>
        </ui-toggle-button>
    `
})
class HostComponent {
    public readonly ctrl = new FormControl<boolean>(false, { nonNullable: true });
}

describe('UiToggleButtonComponent (browser)', () => {
    function button(el: HTMLElement): HTMLButtonElement {
        return el.querySelector('button') as HTMLButtonElement;
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule, ReactiveFormsModule]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return fixture;
    }

    it('projects its content', () => {
        const fixture = setup();
        expect(button(fixture.nativeElement).querySelector('.label')?.textContent).toBe('Filter');
    });

    it('starts unpressed (secondary) and reflects aria-pressed', () => {
        const fixture = setup();
        const btn = button(fixture.nativeElement);
        expect(btn.getAttribute('aria-pressed')).toBe('false');
        expect(btn.classList).toContain('ui-button--secondary');
    });

    it('clicking toggles the control value and the pressed styling', () => {
        const fixture = setup();
        button(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toBe(true);
        const btn = button(fixture.nativeElement);
        expect(btn.getAttribute('aria-pressed')).toBe('true');
        expect(btn.classList).toContain('ui-button--primary');
    });

    it('reflects a programmatic control value as pressed', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.setValue(true);
        fixture.detectChanges();
        expect(button(fixture.nativeElement).getAttribute('aria-pressed')).toBe('true');
    });

    it('disables the button when the control is disabled', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.disable();
        fixture.detectChanges();
        expect(button(fixture.nativeElement).disabled).toBe(true);
    });
});
