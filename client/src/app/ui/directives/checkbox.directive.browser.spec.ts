import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { UiModule } from '../ui.module';

@Component({
    standalone: false,
    template: `
        <input type="checkbox" uiCheckbox [formControl]="ctrl" />
    `
})
class HostComponent {
    public readonly ctrl = new FormControl<boolean>(false, { nonNullable: true });
}

describe('UiCheckboxDirective (browser)', () => {
    function input(el: HTMLElement): HTMLInputElement {
        return el.querySelector('input') as HTMLInputElement;
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

    it('carries the ui-checkbox class', () => {
        const fixture = setup();
        expect(input(fixture.nativeElement).classList).toContain('ui-checkbox');
    });

    it('clicking toggles the form control value', () => {
        const fixture = setup();
        input(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toBe(true);
        input(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toBe(false);
    });

    it('reflects a programmatic control value onto the DOM checked state', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.setValue(true);
        fixture.detectChanges();
        expect(input(fixture.nativeElement).checked).toBe(true);
    });

    it('disables the input when the control is disabled', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.disable();
        fixture.detectChanges();
        expect(input(fixture.nativeElement).disabled).toBe(true);
    });
});
