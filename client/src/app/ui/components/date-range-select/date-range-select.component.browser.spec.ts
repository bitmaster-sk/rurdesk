import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../ui.module';
import { UiDateRangeValue } from './date-range-select.component';

@Component({
    template: `
        <input id="outside" />
        <ui-date-range-select
            inputId="dr"
            [presets]="presets"
            [formControl]="control"
            placeholder="Any time"
        />
    `,
    standalone: false
})
class HostComponent {
    public readonly presets = [
        { label: 'Last 7 days', value: '7d' },
        { label: 'Last 30 days', value: '30d' }
    ];
    public readonly control = new FormControl<UiDateRangeValue | null>(null);
}

describe('UiDateRangeSelectComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule, TranslateModule.forRoot(), UiModule],
            declarations: [HostComponent]
        }).compileComponents();
    });

    afterEach(() => {
        document.querySelectorAll('.cdk-overlay-container').forEach(node => node.remove());
    });

    function render() {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        const el = fixture.nativeElement as HTMLElement;
        const panel = () => document.querySelector<HTMLElement>('.ui-date-range-panel');
        return {
            fixture,
            host: fixture.componentInstance,
            panel,
            trigger: () => el.querySelector<HTMLElement>('.ui-date-range-select__trigger')!,
            triggerText: () => el.querySelector('.ui-select-trigger__value')!.textContent.trim(),
            clearBtn: () => el.querySelector<HTMLButtonElement>('.ui-select-trigger__clear'),
            outside: () => el.querySelector<HTMLInputElement>('#outside')!,
            presetRows: () =>
                Array.from(
                    panel()?.querySelectorAll<HTMLButtonElement>('.ui-date-range-select__preset') ??
                        []
                ),
            calendar: () => panel()?.querySelector('.flatpickr-calendar'),
            open(): void {
                this.trigger().click();
                fixture.detectChanges();
            },
            tick: async (): Promise<void> => {
                await new Promise(resolve => setTimeout(resolve, 0));
                fixture.detectChanges();
            },
            /** flatpickr is a dynamic import — poll instead of guessing a delay. */
            waitFor: async (predicate: () => unknown): Promise<void> => {
                for (let attempt = 0; attempt < 100 && !predicate(); attempt++) {
                    await new Promise(resolve => setTimeout(resolve, 10));
                    fixture.detectChanges();
                }
            },
            dismissOutside(): void {
                const target = el.querySelector<HTMLInputElement>('#outside')!;
                target.focus();
                for (const type of ['pointerdown', 'mousedown', 'click']) {
                    target.dispatchEvent(new MouseEvent(type, { bubbles: true }));
                }
                fixture.detectChanges();
            }
        };
    }

    it('opens the panel on the trigger and lists the presets plus a custom row', () => {
        const page = render();
        expect(page.panel()).toBeNull();

        page.open();

        expect(page.panel()).not.toBeNull();
        expect(page.presetRows().map(row => row.textContent.trim())).toEqual([
            'Last 7 days',
            'Last 30 days',
            'UI.DATE_RANGE.CUSTOM'
        ]);
    });

    it('picking a preset writes the form value and closes the panel', () => {
        const page = render();
        page.open();

        page.presetRows()[1].click();
        page.fixture.detectChanges();

        expect(page.host.control.value).toEqual({ preset: '30d' });
        expect(page.panel()).toBeNull();
        expect(page.triggerText()).toBe('Last 30 days');
    });

    it('shows the placeholder until something is picked', () => {
        expect(render().triggerText()).toBe('Any time');
    });

    it('formats a fixed range on the trigger', () => {
        const page = render();
        page.host.control.setValue({ from: new Date(2026, 0, 1), to: new Date(2026, 0, 31) });
        page.fixture.detectChanges();

        expect(page.triggerText()).toContain('–');
        expect(page.triggerText()).toContain('2026');
    });

    it('falls back to the raw token when no preset matches', () => {
        const page = render();
        page.host.control.setValue({ preset: '1d8h6m' });
        page.fixture.detectChanges();

        expect(page.triggerText()).toBe('1d8h6m');
    });

    // display:none on the flatpickr anchor breaks its wiring, so assert the calendar
    // DOM actually materialises rather than trusting the styling.
    it('renders a real calendar when custom range is chosen', async () => {
        const page = render();
        page.open();

        page.presetRows()[2].click();
        page.fixture.detectChanges();
        await page.waitFor(() => page.calendar());

        expect(page.calendar()).not.toBeNull();
    });

    it('choosing custom range does not change the value on its own', () => {
        const page = render();
        page.open();

        page.presetRows()[2].click();
        page.fixture.detectChanges();

        expect(page.host.control.value).toBeNull();
        expect(page.panel()).not.toBeNull();
    });

    it('the clear button empties the value and hides itself', () => {
        const page = render();
        page.host.control.setValue({ preset: '7d' });
        page.fixture.detectChanges();
        expect(page.clearBtn()).not.toBeNull();

        page.clearBtn()!.click();
        page.fixture.detectChanges();

        expect(page.host.control.value).toBeNull();
        expect(page.clearBtn()).toBeNull();
        expect(page.triggerText()).toBe('Any time');
    });

    // Regression: propagate() used to skip the calendar's control, so a cleared value
    // left the old range highlighted the next time custom range was opened.
    it('clearing also clears the calendar selection', async () => {
        const page = render();
        page.host.control.setValue({ from: new Date(2026, 0, 1), to: new Date(2026, 0, 31) });
        page.fixture.detectChanges();

        page.clearBtn()!.click();
        page.fixture.detectChanges();
        page.open();
        page.presetRows()[2].click();
        page.fixture.detectChanges();
        await page.waitFor(() => page.calendar());

        expect(page.panel()!.querySelectorAll('.flatpickr-day.selected')).toHaveLength(0);
    });

    // Regression: onPanelHide() used to focus the trigger unconditionally, stealing
    // focus from whatever the user clicked to dismiss the panel.
    it('an outside click does not pull focus back to the trigger', async () => {
        const page = render();
        page.open();

        page.dismissOutside();
        await page.waitFor(() => !page.panel());

        expect(page.panel()).toBeNull();
        expect(document.activeElement).toBe(page.outside());
    });

    // Regression: onRangeChange used to swallow the footer Clear as a half-pick,
    // leaving the trigger and filter on the old range while the calendar emptied.
    it('the calendar footer Clear empties the value', async () => {
        const page = render();
        page.host.control.setValue({ from: new Date(2026, 0, 1), to: new Date(2026, 0, 31) });
        page.fixture.detectChanges();

        page.open();
        await page.waitFor(() => page.calendar());
        const clear = page
            .panel()!
            .querySelectorAll<HTMLButtonElement>('.ui-datepicker-footer__btn')[1];
        clear.click();
        page.fixture.detectChanges();

        expect(page.host.control.value).toBeNull();
        expect(page.triggerText()).toBe('Any time');
        expect(page.panel()).not.toBeNull();
    });

    it('disabling while the panel is open closes it', async () => {
        const page = render();
        page.open();
        expect(page.panel()).not.toBeNull();

        page.host.control.disable();
        page.fixture.detectChanges();
        await page.waitFor(() => !page.panel());

        expect(page.panel()).toBeNull();
    });

    it('ArrowDown opens the panel and walks the preset rows, wrapping at the end', async () => {
        const page = render();
        page.trigger().focus();
        page.trigger().dispatchEvent(
            new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })
        );
        page.fixture.detectChanges();
        await page.waitFor(() => document.activeElement === page.presetRows()[0]);
        expect(document.activeElement).toBe(page.presetRows()[0]);

        const step = async (key: string, index: number): Promise<void> => {
            document.activeElement!.dispatchEvent(
                new KeyboardEvent('keydown', { key, bubbles: true })
            );
            page.fixture.detectChanges();
            await page.waitFor(() => document.activeElement === page.presetRows()[index]);
            expect(document.activeElement).toBe(page.presetRows()[index]);
        };

        await step('ArrowDown', 1);
        await step('ArrowDown', 2);
        await step('ArrowDown', 0); // wraps
        await step('ArrowUp', 2); // and back
    });

    // Regression: only the ArrowDown path moved focus into the panel, so opening with
    // Enter left focus on the trigger and the next arrow escaped to the global
    // list hotkeys — the task selection moved instead of the preset.
    it.each(['Enter', ' '])('opening with %s focuses the first preset', async key => {
        const page = render();
        page.trigger().focus();
        page.trigger().dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
        page.fixture.detectChanges();
        await page.waitFor(() => document.activeElement === page.presetRows()[0]);

        expect(page.panel()).not.toBeNull();
        expect(document.activeElement).toBe(page.presetRows()[0]);
    });

    it('keys the trigger consumes never reach the document', () => {
        const page = render();
        const seen: string[] = [];
        const listener = (e: Event): void => seen.push((e as KeyboardEvent).key);
        document.addEventListener('keydown', listener);

        page.trigger().focus();
        for (const key of ['ArrowDown', 'ArrowUp', 'Enter', ' ']) {
            page.trigger().dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
            page.fixture.detectChanges();
        }
        document.removeEventListener('keydown', listener);

        expect(seen).toEqual([]);
    });

    it('a disabled control cannot open the panel', () => {
        const page = render();
        page.host.control.disable();
        page.fixture.detectChanges();

        page.open();

        expect(page.panel()).toBeNull();
    });

    it('writeValue does not echo back into the form', () => {
        const page = render();
        const spy = vi.fn();
        page.host.control.valueChanges.subscribe(spy);

        page.host.control.setValue({ preset: '7d' }, { emitEvent: false });
        page.fixture.detectChanges();

        expect(spy).not.toHaveBeenCalled();
    });
});
