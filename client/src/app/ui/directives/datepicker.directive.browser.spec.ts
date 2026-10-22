import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { Component, Input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../ui.module';

@Component({
    standalone: false,
    template: `
        <input uiDatepicker [mode]="mode" [formControl]="control" />
    `
})
class HostComponent {
    public mode: 'date' | 'datetime' | 'range' = 'date';
    public readonly control = new FormControl<Date | Date[] | null>(null);
}

@Component({
    standalone: false,
    template: `
        <input uiDatepicker mode="date" [maxDate]="maxDate" [formControl]="control" />
    `
})
class MaxDateHostComponent {
    @Input() public maxDate: Date | null = null;
    public readonly control = new FormControl<Date | null>(null);
}

/** Wait until flatpickr has attached to the input and the calendar exists. */
async function waitForPicker(input: HTMLInputElement): Promise<void> {
    for (let i = 0; i < 100; i++) {
        if ((input as unknown as { _flatpickr?: unknown })._flatpickr) return;
        await new Promise(r => setTimeout(r, 10));
    }
    throw new Error('flatpickr did not attach');
}

function fpInstance(input: HTMLInputElement): {
    setDate: (d: Date | Date[], trigger?: boolean) => void;
    clear: () => void;
    open: () => void;
    calendarContainer: HTMLElement;
} {
    return (input as unknown as { _flatpickr: never })._flatpickr;
}

async function openCalendar(input: HTMLInputElement): Promise<void> {
    await waitForPicker(input);
    fpInstance(input).open();
    await new Promise(r => setTimeout(r, 0));
}

function dayCell(container: HTMLElement, day: number): HTMLElement | null {
    return (
        Array.from(
            container.querySelectorAll('.flatpickr-day:not(.prevMonthDay):not(.nextMonthDay)')
        ).find((el): el is HTMLElement => el.textContent === String(day)) ?? null
    );
}

describe('UiDatepickerDirective (flatpickr)', () => {
    let fixture: ComponentFixture<HostComponent>;
    let host: HostComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [HostComponent, MaxDateHostComponent],
            imports: [UiModule, ReactiveFormsModule, TranslateModule.forRoot()]
        }).compileComponents();
        fixture = TestBed.createComponent(HostComponent);
        host = fixture.componentInstance;
    });

    afterEach(() => fixture.destroy());

    function input(): HTMLInputElement {
        return fixture.nativeElement.querySelector('input') as HTMLInputElement;
    }

    it('renders an initial value written BEFORE flatpickr init (buffered writeValue)', async () => {
        host.control.setValue(new Date(2026, 6, 3)); // set before first CD / view init
        fixture.detectChanges();
        await waitForPicker(input());
        fixture.detectChanges();
        expect(input().value).toBe('Jul 3, 2026');
    });

    it('propagates a picked date to the control', async () => {
        fixture.detectChanges();
        await waitForPicker(input());
        fpInstance(input()).setDate(new Date(2026, 6, 3), true); // true → fires onChange
        await fixture.whenStable();
        expect(host.control.value).toBeInstanceOf(Date);
        expect((host.control.value as Date).getDate()).toBe(3);
    });

    it('Clear sets the control to null', async () => {
        host.control.setValue(new Date(2026, 6, 3));
        fixture.detectChanges();
        await waitForPicker(input());
        fpInstance(input()).clear();
        await fixture.whenStable();
        expect(host.control.value).toBeNull();
    });

    it('range: one date does NOT propagate, two dates propagate [from,to]', async () => {
        host.mode = 'range';
        fixture.detectChanges();
        await waitForPicker(input());
        const fp = fpInstance(input());

        fp.setDate([new Date(2026, 6, 3)], true); // partial range
        await fixture.whenStable();
        expect(host.control.value).toBeNull(); // suppressed

        fp.setDate([new Date(2026, 6, 3), new Date(2026, 6, 10)], true);
        await fixture.whenStable();
        expect(Array.isArray(host.control.value)).toBe(true);
        expect((host.control.value as Date[]).length).toBe(2);
    });

    it('disables future days when maxDate is today', async () => {
        const maxFixture = TestBed.createComponent(MaxDateHostComponent);
        const maxHost = maxFixture.componentInstance;
        const today = new Date(2026, 6, 3);
        const tomorrow = new Date(2026, 6, 4);
        maxHost.maxDate = today;
        maxHost.control.setValue(today);
        maxFixture.detectChanges();
        await openCalendar(maxFixture.nativeElement.querySelector('input'));
        const cell = dayCell(
            fpInstance(maxFixture.nativeElement.querySelector('input')).calendarContainer,
            tomorrow.getDate()
        );
        expect(cell).not.toBeNull();
        expect(cell!.classList.contains('flatpickr-disabled')).toBe(true);
        maxFixture.destroy();
    });

    it('keeps future days enabled when maxDate is not set', async () => {
        const maxFixture = TestBed.createComponent(MaxDateHostComponent);
        const maxHost = maxFixture.componentInstance;
        const today = new Date(2026, 6, 3);
        const tomorrow = new Date(2026, 6, 4);
        maxHost.control.setValue(today);
        maxFixture.detectChanges();
        await openCalendar(maxFixture.nativeElement.querySelector('input'));
        const cell = dayCell(
            fpInstance(maxFixture.nativeElement.querySelector('input')).calendarContainer,
            tomorrow.getDate()
        );
        expect(cell).not.toBeNull();
        expect(cell!.classList.contains('flatpickr-disabled')).toBe(false);
        maxFixture.destroy();
    });

    it('disables future days when maxDate is set after the calendar is open', async () => {
        const maxFixture = TestBed.createComponent(MaxDateHostComponent);
        const maxHost = maxFixture.componentInstance;
        const today = new Date(2026, 6, 3);
        const tomorrow = new Date(2026, 6, 4);
        maxHost.control.setValue(today);
        maxFixture.detectChanges();
        await openCalendar(maxFixture.nativeElement.querySelector('input'));
        maxFixture.componentRef.setInput('maxDate', today);
        maxFixture.detectChanges();
        const cell = dayCell(
            fpInstance(maxFixture.nativeElement.querySelector('input')).calendarContainer,
            tomorrow.getDate()
        );
        expect(cell).not.toBeNull();
        expect(cell!.classList.contains('flatpickr-disabled')).toBe(true);
        maxFixture.destroy();
    });

    it('re-enables future days when maxDate is cleared after the calendar is open', async () => {
        const maxFixture = TestBed.createComponent(MaxDateHostComponent);
        const maxHost = maxFixture.componentInstance;
        const today = new Date(2026, 6, 3);
        const tomorrow = new Date(2026, 6, 4);
        maxHost.maxDate = today;
        maxHost.control.setValue(today);
        maxFixture.detectChanges();
        await openCalendar(maxFixture.nativeElement.querySelector('input'));
        maxFixture.componentRef.setInput('maxDate', null);
        maxFixture.detectChanges();
        const cell = dayCell(
            fpInstance(maxFixture.nativeElement.querySelector('input')).calendarContainer,
            tomorrow.getDate()
        );
        expect(cell).not.toBeNull();
        expect(cell!.classList.contains('flatpickr-disabled')).toBe(false);
        maxFixture.destroy();
    });
});
