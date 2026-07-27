import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { UiModule } from '../../ui.module';

interface Opt {
    label: string;
    value: string;
}

@Component({
    standalone: false,
    template: `
        <ui-choice
            [options]="options()"
            optionLabel="label"
            optionValue="value"
            [allowEmpty]="allowEmpty()"
            [formControl]="ctrl"
        >
            @if (withTemplate()) {
                <ng-template #item let-item>tpl:{{ item.label }}</ng-template>
            }
        </ui-choice>
    `
})
class HostComponent {
    public readonly options = signal<Opt[]>([
        { label: 'Columns', value: 'columns' },
        { label: 'Swimlane', value: 'swimlane' }
    ]);
    public readonly ctrl = new FormControl<string | null>(null);
    public readonly allowEmpty = signal(true);
    public readonly withTemplate = signal(false);
}

describe('UiChoiceComponent (browser)', () => {
    function options(el: HTMLElement): HTMLButtonElement[] {
        return Array.from(el.querySelectorAll('.ui-choice__option'));
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

    it('renders one button per option with resolved labels', () => {
        const fixture = setup();
        expect(options(fixture.nativeElement).map(b => b.textContent?.trim())).toEqual([
            'Columns',
            'Swimlane'
        ]);
    });

    it('clicking an option writes its optionValue to the form control', () => {
        const fixture = setup();
        options(fixture.nativeElement)[1].click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toBe('swimlane');
    });

    it('marks only the selected option (single-select exclusivity)', () => {
        const fixture = setup();
        options(fixture.nativeElement)[0].click();
        fixture.detectChanges();
        const buttons = options(fixture.nativeElement);
        expect(buttons[0].classList).toContain('ui-button--primary');
        expect(buttons[1].classList).not.toContain('ui-button--primary');
        expect(buttons[0].getAttribute('aria-pressed')).toBe('true');
    });

    it('re-clicking the selected option clears it when allowEmpty=true', () => {
        const fixture = setup();
        options(fixture.nativeElement)[0].click();
        fixture.detectChanges();
        options(fixture.nativeElement)[0].click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toBeNull();
    });

    it('re-clicking the selected option keeps it when allowEmpty=false', () => {
        const fixture = setup();
        fixture.componentInstance.allowEmpty.set(false);
        fixture.detectChanges();
        options(fixture.nativeElement)[0].click();
        fixture.detectChanges();
        options(fixture.nativeElement)[0].click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toBe('columns');
    });

    it('renders a projected item template when provided', () => {
        const fixture = setup();
        fixture.componentInstance.withTemplate.set(true);
        fixture.detectChanges();
        expect(options(fixture.nativeElement)[0].textContent?.trim()).toBe('tpl:Columns');
    });

    it('reflects a programmatic control value as the selected option', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.setValue('swimlane');
        fixture.detectChanges();
        const buttons = options(fixture.nativeElement);
        expect(buttons[1].classList).toContain('ui-button--primary');
    });

    it('disables all option buttons when the control is disabled', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.disable();
        fixture.detectChanges();
        expect(options(fixture.nativeElement).every(b => b.disabled)).toBe(true);
    });
});
