import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { addDays, startOfDay, subDays } from 'date-fns';
import { UiModule } from '../../ui.module';
import { UI_DATE_PATTERN, UiDate } from '../../util/ui-date';
import { UiDaySelectFormat } from './day-select-label';

@Component({
    template: `
        <input id="outside" />
        <ui-day-select
            [maxDate]="today"
            [format]="format"
            [fluid]="fluid"
            [formControl]="control"
        />
    `,
    standalone: false
})
class HostComponent {
    public readonly control = new FormControl<Date | null>(startOfDay(new Date()));
    public readonly today = new Date();
    public format = UiDaySelectFormat.Short;
    public fluid = false;
}

describe('UiDaySelectComponent (browser)', () => {
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
        const panel = () => document.querySelector<HTMLElement>('.ui-day-select-panel');
        return {
            fixture,
            host: fixture.componentInstance,
            panel,
            trigger: () => el.querySelector<HTMLButtonElement>('button.ui-day-select__trigger')!,
            triggerText: () => el.querySelector('.ui-day-select__value')!.textContent.trim(),
            presetRows: () =>
                Array.from(
                    panel()?.querySelectorAll<HTMLButtonElement>('.ui-day-select__preset') ?? []
                ),
            calendar: () => panel()?.querySelector('.flatpickr-calendar'),
            dayCell: (day: Date) =>
                panel()?.querySelector<HTMLElement>(
                    `.flatpickr-day[aria-label="${UiDate.format(day, UI_DATE_PATTERN)}"]`
                ),
            open(): void {
                this.trigger().click();
                fixture.detectChanges();
            },
            tick: async (): Promise<void> => {
                await new Promise(resolve => setTimeout(resolve, 0));
                fixture.detectChanges();
            },
            waitFor: async (predicate: () => unknown): Promise<void> => {
                for (let attempt = 0; attempt < 100 && !predicate(); attempt++) {
                    await new Promise(resolve => setTimeout(resolve, 10));
                    fixture.detectChanges();
                }
            }
        };
    }

    it('labels a today value Today without the past state', () => {
        const page = render();

        expect(page.triggerText()).toBe('UI.DAY_SELECT.TODAY');
        expect(page.trigger().classList).not.toContain('ui-day-select--past');
    });

    it('picking Yesterday writes yesterday 00:00, closes the popover and flags the trigger', () => {
        const page = render();
        page.open();
        expect(page.panel()).not.toBeNull();

        page.presetRows()[1].click();
        page.fixture.detectChanges();

        expect(page.host.control.value).toEqual(startOfDay(subDays(new Date(), 1)));
        expect(page.panel()).toBeNull();
        expect(page.triggerText()).toBe('UI.DAY_SELECT.YESTERDAY');
        expect(page.trigger().classList).toContain('ui-day-select--past');
    });

    it('presses only the preset matching the current value', () => {
        const page = render();
        page.open();

        expect(page.presetRows()[0].getAttribute('aria-pressed')).toBe('true');
        expect(page.presetRows()[1].getAttribute('aria-pressed')).toBe('false');

        page.host.control.setValue(startOfDay(subDays(new Date(), 1)));
        page.fixture.detectChanges();

        expect(page.presetRows()[0].getAttribute('aria-pressed')).toBe('false');
        expect(page.presetRows()[1].getAttribute('aria-pressed')).toBe('true');
    });

    it('formats the trigger with EEE, MMM d when format is Long', () => {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentInstance.control.setValue(new Date(2026, 9, 19));
        fixture.componentInstance.format = UiDaySelectFormat.Long;
        fixture.detectChanges();
        const el = fixture.nativeElement as HTMLElement;

        expect(el.querySelector('.ui-day-select__value')!.textContent.trim()).toBe('Mon, Oct 19');
    });

    it('marks the trigger fluid only when the fluid input is set', () => {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        const el = fixture.nativeElement as HTMLElement;
        const trigger = () => el.querySelector<HTMLElement>('button.ui-day-select__trigger')!;

        expect(trigger().classList).not.toContain('ui-day-select--fluid');

        fixture.componentInstance.fluid = true;
        fixture.changeDetectorRef.markForCheck();
        fixture.detectChanges();

        expect(trigger().classList).toContain('ui-day-select--fluid');
    });

    it('formats an older day as EEE d in the short format', () => {
        const page = render();
        page.host.control.setValue(new Date(2026, 9, 16));
        page.fixture.detectChanges();

        expect(page.triggerText()).toBe('Fri 16');
        expect(page.trigger().classList).toContain('ui-day-select--past');
    });

    it('a value written from outside shows on the trigger', () => {
        const page = render();
        const day = startOfDay(subDays(new Date(), 2));

        page.host.control.setValue(day);
        page.fixture.detectChanges();

        expect(page.triggerText()).toBe(UiDate.format(day, 'EEE d'));
        expect(page.trigger().classList).toContain('ui-day-select--past');
    });

    it('picking a day in the calendar closes the popover and restores focus', async () => {
        const page = render();
        page.open();
        await page.waitFor(() => page.calendar());

        page.dayCell(startOfDay(new Date()))!.click();
        await page.tick();

        expect(page.host.control.value).toEqual(startOfDay(new Date()));
        expect(page.panel()).toBeNull();
        expect(page.triggerText()).toBe('UI.DAY_SELECT.TODAY');
        expect(document.activeElement).toBe(page.trigger());
    });

    it('leaves tomorrow unpickable in the calendar while maxDate is today', async () => {
        const page = render();
        page.open();
        await page.waitFor(() => page.calendar());

        const tomorrow = page.dayCell(startOfDay(addDays(new Date(), 1)))!;
        expect(tomorrow.classList).toContain('flatpickr-disabled');

        const before = page.host.control.value;
        tomorrow.click();
        await page.tick();

        expect(page.host.control.value).toBe(before);
        expect(page.panel()).not.toBeNull();
    });

    it('Escape closes the popover and restores focus to the trigger', async () => {
        const page = render();
        page.open();
        await page.waitFor(() => page.calendar());
        page.presetRows()[0].focus();

        page.panel()!
            .querySelector('[role="dialog"]')!
            .dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        page.fixture.detectChanges();
        await page.waitFor(() => !page.panel());

        expect(page.panel()).toBeNull();
        expect(page.trigger().getAttribute('aria-expanded')).toBe('false');
        expect(document.activeElement).toBe(page.trigger());
    });

    it('writeValue does not echo back into the form', () => {
        const page = render();
        const spy = vi.fn();
        page.host.control.valueChanges.subscribe(spy);

        page.host.control.setValue(startOfDay(subDays(new Date(), 1)), { emitEvent: false });
        page.fixture.detectChanges();

        expect(spy).not.toHaveBeenCalled();
    });

    it('a disabled control cannot open the popover', () => {
        const page = render();
        page.host.control.disable();
        page.fixture.detectChanges();

        page.open();

        expect(page.panel()).toBeNull();
    });
});
