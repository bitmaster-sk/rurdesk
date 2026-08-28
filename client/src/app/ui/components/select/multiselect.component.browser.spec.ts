import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../ui.module';

interface Opt {
    label: string;
    value: string;
}

@Component({
    standalone: false,
    template: `
        <ui-multiselect
            [options]="options()"
            optionLabel="label"
            optionValue="value"
            placeholder="Pick many"
            [formControl]="ctrl"
            (valueChanged)="onChangeCount = onChangeCount + 1"
        />
    `
})
class HostComponent {
    public readonly options = signal<Opt[] | null>([
        { label: 'Alpha', value: 'a' },
        { label: 'Beta', value: 'b' },
        { label: 'Gamma', value: 'c' }
    ]);
    public readonly ctrl = new FormControl<string[]>([], { nonNullable: true });
    public onChangeCount = 0;
}

describe('UiMultiSelectComponent (browser)', () => {
    function trigger(el: HTMLElement): HTMLElement {
        return el.querySelector('.ui-select-trigger') as HTMLElement;
    }
    function options(): HTMLElement[] {
        return Array.from(document.querySelectorAll('.ui-select-panel__option'));
    }
    function header(): HTMLElement | null {
        return document.querySelector('.ui-select-panel__header .ui-select-panel__checkbox');
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent],
            imports: [UiModule, ReactiveFormsModule, TranslateModule.forRoot()],
            providers: [provideNoopAnimations()]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return fixture;
    }

    it('toggles options into the array and stays open', () => {
        const fixture = setup();
        trigger(fixture.nativeElement).click();
        fixture.detectChanges();

        options()[0].click();
        fixture.detectChanges();
        options()[2].click();
        fixture.detectChanges();

        expect(fixture.componentInstance.ctrl.value).toEqual(['a', 'c']);
        expect(options().length).toBe(3); // still open
        expect(fixture.componentInstance.onChangeCount).toBe(2);
    });

    it('removes an already-selected option on second toggle', () => {
        const fixture = setup();
        trigger(fixture.nativeElement).click();
        fixture.detectChanges();
        options()[0].click();
        fixture.detectChanges();
        options()[0].click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toEqual([]);
    });

    it('shows a comma-joined label in the trigger (no selectedItems template)', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.setValue(['a', 'b']);
        fixture.detectChanges();
        expect(trigger(fixture.nativeElement).textContent).toContain('Alpha, Beta');
    });

    it('does NOT emit onChange on writeValue', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.setValue(['a']);
        fixture.detectChanges();
        expect(fixture.componentInstance.onChangeCount).toBe(0);
    });

    it('select-all header toggles all, then clears (tri-state)', () => {
        const fixture = setup();
        trigger(fixture.nativeElement).click();
        fixture.detectChanges();

        header()!.click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toEqual(['a', 'b', 'c']);
        expect(header()!.classList).toContain('ui-select-panel__checkbox--checked');

        header()!.click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toEqual([]);
    });

    it('select-all is reachable by keyboard: Ctrl+A toggles all, then clears', () => {
        const fixture = setup();
        const el = trigger(fixture.nativeElement);
        el.click();
        fixture.detectChanges();

        el.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', ctrlKey: true, bubbles: true }));
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toEqual(['a', 'b', 'c']);

        el.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', ctrlKey: true, bubbles: true }));
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toEqual([]);
    });

    it('Ctrl+A does nothing while the panel is closed', () => {
        const fixture = setup();
        const el = trigger(fixture.nativeElement);

        el.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', ctrlKey: true, bubbles: true }));
        fixture.detectChanges();

        expect(fixture.componentInstance.ctrl.value).toEqual([]);
    });

    it('survives null options, as an async pipe hands over before its first emission', () => {
        const fixture = setup();

        fixture.componentInstance.options.set(null);
        fixture.detectChanges();

        expect(trigger(fixture.nativeElement)).toBeTruthy();
    });

    it('header is indeterminate on a partial selection', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.setValue(['a']);
        fixture.detectChanges();
        trigger(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(header()!.classList).toContain('ui-select-panel__checkbox--indeterminate');
    });
});
