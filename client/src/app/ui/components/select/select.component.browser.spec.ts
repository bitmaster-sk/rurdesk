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
        <ui-select
            [options]="options()"
            optionLabel="label"
            optionValue="value"
            placeholder="Pick one"
            [formControl]="ctrl"
            (valueChanged)="onChangeCount = onChangeCount + 1"
        />
    `
})
class HostComponent {
    public readonly options = signal<Opt[] | null>([]);
    public readonly ctrl = new FormControl<string | null>(null);
    public onChangeCount = 0;
}

@Component({
    standalone: false,
    template: `
        <ui-select
            [options]="options()"
            optionLabel="label"
            optionValue="value"
            placeholder="Pick one"
            [showClear]="true"
            [formControl]="ctrl"
        />
    `
})
class ClearHostComponent {
    public readonly options = signal<Opt[]>([
        { label: 'Alpha', value: 'a' },
        { label: 'Beta', value: 'b' }
    ]);
    public readonly ctrl = new FormControl<string | null>('a');
}

@Component({
    standalone: false,
    template: `
        <ui-select
            [options]="options()"
            optionLabel="label"
            optionValue="value"
            [formControl]="ctrl"
        >
            <ng-template #optionActions let-openDock="openDock">
                <button type="button" class="row-action" (click)="openDock('info')">i</button>
            </ng-template>
            <ng-template #dock let-opt>
                <p class="row-dock">dock for {{ opt.label }}</p>
            </ng-template>
        </ui-select>
    `
})
class ActionsHostComponent {
    public readonly options = signal<Opt[]>([
        { label: 'Alpha', value: 'a' },
        { label: 'Beta', value: 'b' }
    ]);
    public readonly ctrl = new FormControl<string | null>(null);
}

describe('UiSelectComponent (browser)', () => {
    function trigger(el: HTMLElement): HTMLElement {
        return el.querySelector('.ui-select-trigger') as HTMLElement;
    }
    function panelOptions(): HTMLElement[] {
        return Array.from(document.querySelectorAll('.ui-select-panel__option'));
    }
    function triggerText(el: HTMLElement): string {
        return trigger(el).textContent?.trim() ?? '';
    }

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent, ClearHostComponent, ActionsHostComponent],
            imports: [UiModule, ReactiveFormsModule, TranslateModule.forRoot()],
            providers: [provideNoopAnimations()]
        }).compileComponents();
    });

    function setup(
        options: Opt[] = [
            { label: 'Alpha', value: 'a' },
            { label: 'Beta', value: 'b' }
        ]
    ) {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentInstance.options.set(options);
        fixture.detectChanges();
        return fixture;
    }

    it('shows the placeholder when no value', () => {
        const fixture = setup();
        expect(triggerText(fixture.nativeElement)).toContain('Pick one');
    });

    it('renders nothing beyond the trigger', () => {
        const fixture = setup();
        const host = fixture.nativeElement.querySelector('ui-select') as HTMLElement;

        expect(host.textContent?.trim()).toBe('Pick one');
    });

    it('resolves the label reactively even when the value is set before options load', () => {
        const fixture = TestBed.createComponent(HostComponent);
        // value first, options empty
        fixture.componentInstance.ctrl.setValue('b');
        fixture.detectChanges();
        expect(triggerText(fixture.nativeElement)).toContain('Pick one'); // no match yet
        // options arrive later
        fixture.componentInstance.options.set([{ label: 'Beta', value: 'b' }]);
        fixture.detectChanges();
        expect(triggerText(fixture.nativeElement)).toContain('Beta');
    });

    it('does NOT emit onChange on writeValue (programmatic set)', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.setValue('a');
        fixture.detectChanges();
        expect(fixture.componentInstance.onChangeCount).toBe(0);
    });

    it('opens on click and lists the options', () => {
        const fixture = setup();
        expect(panelOptions().length).toBe(0);
        trigger(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(panelOptions().map(o => o.textContent?.trim())).toEqual(['Alpha', 'Beta']);
    });

    it('picking an option updates the form value, emits onChange, and closes', () => {
        const fixture = setup();
        trigger(fixture.nativeElement).click();
        fixture.detectChanges();

        panelOptions()[1].click();
        fixture.detectChanges();

        expect(fixture.componentInstance.ctrl.value).toBe('b');
        expect(fixture.componentInstance.onChangeCount).toBe(1);
        expect(panelOptions().length).toBe(0); // closed
        expect(triggerText(fixture.nativeElement)).toContain('Beta');
    });

    it('does not open when disabled', () => {
        const fixture = setup();
        fixture.componentInstance.ctrl.disable();
        fixture.detectChanges();
        trigger(fixture.nativeElement).click();
        fixture.detectChanges();
        expect(panelOptions().length).toBe(0);
        expect(trigger(fixture.nativeElement).classList).toContain('ui-select-trigger--disabled');
    });

    it('keyboard: ArrowDown opens, then Enter selects the highlighted option', () => {
        const fixture = setup();
        const t = trigger(fixture.nativeElement);
        t.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
        fixture.detectChanges();
        expect(panelOptions().length).toBe(2);

        t.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toBe('a'); // first option highlighted on open
        expect(fixture.componentInstance.onChangeCount).toBe(1);
    });

    it('keyboard: Enter on the clear button clears instead of opening the panel', () => {
        const fixture = TestBed.createComponent(ClearHostComponent);
        fixture.detectChanges();

        const clear = fixture.nativeElement.querySelector(
            '.ui-select-trigger__clear'
        ) as HTMLButtonElement;
        expect(clear).toBeTruthy();

        clear.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        fixture.detectChanges();
        expect(panelOptions().length).toBe(0);

        clear.click();
        fixture.detectChanges();
        expect(fixture.componentInstance.ctrl.value).toBeNull();
    });

    it('keyboard: Tab moves into the highlighted row actions instead of closing', () => {
        const fixture = TestBed.createComponent(ActionsHostComponent);
        fixture.detectChanges();
        const t = trigger(fixture.nativeElement);

        t.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
        fixture.detectChanges();
        expect(panelOptions().length).toBe(2);

        t.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));
        fixture.detectChanges();

        expect(panelOptions().length).toBe(2);
        expect(document.activeElement?.classList.contains('row-action')).toBe(true);
    });

    it('only the highlighted row exposes its actions to the tab order', () => {
        const fixture = TestBed.createComponent(ActionsHostComponent);
        fixture.detectChanges();
        const t = trigger(fixture.nativeElement);

        t.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
        fixture.detectChanges();

        const actions = Array.from(
            document.querySelectorAll<HTMLElement>('.ui-select-panel__actions')
        );
        expect(actions.length).toBe(2);
        expect(actions[0].hasAttribute('inert')).toBe(false);
        expect(actions[1].hasAttribute('inert')).toBe(true);
    });

    it('Escape from inside the panel closes it once focus has left the trigger', () => {
        const fixture = TestBed.createComponent(ActionsHostComponent);
        fixture.detectChanges();
        const t = trigger(fixture.nativeElement);

        t.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
        fixture.detectChanges();
        t.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));
        fixture.detectChanges();

        const action = document.activeElement as HTMLElement;
        action.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        fixture.detectChanges();
        expect(panelOptions().length).toBe(0);
    });

    it('survives null options, as an async pipe hands over before its first emission', () => {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentInstance.options.set(null);
        fixture.detectChanges();

        expect(trigger(fixture.nativeElement)).toBeTruthy();
    });
});
