import {
    AfterViewInit,
    Directive,
    ElementRef,
    NgZone,
    OnDestroy,
    booleanAttribute,
    forwardRef,
    inject,
    input,
    output
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DestroyRef } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { startOfDay } from 'date-fns';
import {
    UI_DATETIME_PATTERN,
    UI_DATE_PATTERN,
    uiFormatDate,
    uiParseDate
} from '../util/ui-date-format';

// Type-only imports (no runtime cost — flatpickr itself is loaded lazily).
type FlatpickrInstance = import('flatpickr/dist/types/instance').Instance;
type FlatpickrOptions = import('flatpickr/dist/types/options').Options;

/** What the field picks. How it is presented is the separate `inline` input. */
type UiDatepickerMode = 'date' | 'datetime' | 'range';

/** Value the CVA exchanges with the form: single Date (date/datetime) or [from,to] (range). */
type UiDatepickerValue = Date | Date[] | null;

/**
 * Applies a flatpickr-backed date/time picker to a native `<input>`, reusing
 * the `.ui-input` chrome (host class) so the field looks like any other
 * `[uiInput]`. Value type is native `Date` (range = `[from, to]`). flatpickr is
 * loaded lazily and its calendar/format are themed via `--ui-datepicker-*`
 * tokens + a global override (see styles.scss).
 */
@Directive({
    selector: 'input[uiDatepicker]',
    standalone: false,
    host: {
        'class': 'ui-input',
        'readonly': 'true', // allowInput:false → the field is picked, not typed
        '[class.ui-input--invalid]': 'invalid()'
    },
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UiDatepickerDirective),
            multi: true
        }
    ]
})
export class UiDatepickerDirective implements ControlValueAccessor, AfterViewInit, OnDestroy {
    private readonly el = inject<ElementRef<HTMLInputElement>>(ElementRef);
    private readonly zone = inject(NgZone);
    private readonly i18n = inject(I18nService);
    private readonly destroyRef = inject(DestroyRef);

    /** `date` (default) · `datetime` (12h) · `range` ([from,to]). */
    public readonly mode = input<UiDatepickerMode>('date');
    /** Calendar in-flow instead of a popup. Combines with any mode. */
    public readonly inline = input(false, { transform: booleanAttribute });
    /** Invalid styling, mirrors `[uiInput]`. */
    public readonly invalid = input(false, { transform: booleanAttribute });

    /** Fires once flatpickr has built its calendar DOM. Lets a host overlay
     *  reposition after the (lazily-imported) calendar actually exists — otherwise
     *  it stays anchored to the pre-calendar size and the calendar overflows. */
    public readonly ready = output<void>();

    private fp: FlatpickrInstance | null = null;
    private destroyed = false;

    private onChange: (value: UiDatepickerValue) => void = () => {};
    private onTouch: () => void = () => {};

    // Buffered state written before the (lazily-loaded) flatpickr instance exists.
    private pending: UiDatepickerValue | undefined = undefined;
    private pendingDisabled = false;
    // Last value we propagated outward — used to revert a half-finished range on close.
    private lastValue: UiDatepickerValue = null;

    public async ngAfterViewInit(): Promise<void> {
        const flatpickr = (await import('flatpickr')).default;
        if (this.destroyed) {
            return;
        }
        this.zone.runOutsideAngular(() => {
            this.fp = flatpickr(this.el.nativeElement, this.buildOptions());
        });
        if (this.pending !== undefined) {
            this.fp!.setDate(this.toSetDate(this.pending), false);
            this.pending = undefined;
        }
        if (this.pendingDisabled) {
            this.applyDisabled(true);
        }
        this.ready.emit();
    }

    public ngOnDestroy(): void {
        this.destroyed = true;
        this.fp?.destroy();
    }

    // ── ControlValueAccessor ────────────────────────────────────────────────

    public writeValue(value: UiDatepickerValue): void {
        this.lastValue = value;
        if (this.fp) {
            this.fp.setDate(this.toSetDate(value), false); // 2nd arg false → no onChange (no loop)
        } else {
            this.pending = value; // buffer until the instance exists
        }
    }

    public registerOnChange(fn: (value: UiDatepickerValue) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouch = fn;
    }

    public setDisabledState(isDisabled: boolean): void {
        if (this.fp) {
            this.applyDisabled(isDisabled);
        } else {
            this.pendingDisabled = isDisabled;
        }
    }

    // ── internals ─────────────────────────────────────────────────────────────

    private buildOptions(): FlatpickrOptions {
        const mode = this.mode();
        const isDatetime = mode === 'datetime';
        const isRange = mode === 'range';
        const pattern = isDatetime ? UI_DATETIME_PATTERN : UI_DATE_PATTERN;

        return {
            disableMobile: true, // never fall back to the native (unstyled) picker
            allowInput: false, // pick, don't type → parseDate never load-bearing
            enableTime: isDatetime, // range never combines with time (unsupported upstream)
            time_24hr: false, // locale/12h
            mode: isRange ? 'range' : 'single',
            inline: this.inline(),
            formatDate: (date: Date) => uiFormatDate(date, pattern),
            parseDate: (str: string) => uiParseDate(str, pattern),
            onChange: (selectedDates: Date[]) => this.zone.run(() => this.emit(selectedDates)),
            onClose: (selectedDates: Date[]) =>
                this.zone.run(() => this.handleClose(selectedDates)),
            onReady: (_sel: Date[], _str: string, instance: FlatpickrInstance) =>
                this.attachFooter(instance)
        };
    }

    private emit(selectedDates: Date[]): void {
        if (this.mode() === 'range') {
            if (selectedDates.length === 0) {
                this.propagate(null); // Clear
            } else if (selectedDates.length === 2) {
                this.propagate([selectedDates[0], selectedDates[1]]);
            }
            // length 1 = half-finished range → suppress (don't fire the filter early)
            return;
        }
        this.propagate(selectedDates[0] ?? null);
    }

    private handleClose(selectedDates: Date[]): void {
        this.onTouch();
        // A range popup closed mid-selection would leave the input showing a single
        // date that isn't in the control value → revert to the last propagated value.
        if (this.mode() === 'range' && selectedDates.length === 1 && this.fp) {
            this.fp.setDate(this.toSetDate(this.lastValue), false);
        }
    }

    private propagate(value: UiDatepickerValue): void {
        this.lastValue = value;
        this.onChange(value);
    }

    // Normalize any control value into what flatpickr.setDate accepts, stripping
    // null/undefined members from partial ranges (`[Date, null]`).
    private toSetDate(value: UiDatepickerValue): Date | Date[] {
        if (value == null) {
            return [];
        }
        if (Array.isArray(value)) {
            return value.filter((d): d is Date => d instanceof Date);
        }
        return value;
    }

    private applyDisabled(isDisabled: boolean): void {
        this.el.nativeElement.disabled = isDisabled;
        this.fp?.set('clickOpens', !isDisabled);
    }

    // Today / Clear footer. Appended to calendarContainer (its direct children
    // survive flatpickr's redraws, which only rewrite the day/month innerHTML).
    private attachFooter(instance: FlatpickrInstance): void {
        const footer = this.el.nativeElement.ownerDocument.createElement('div');
        footer.className = 'ui-datepicker-footer';

        const today = this.makeFooterButton();
        const clear = this.makeFooterButton();

        const setLabels = (): void => {
            today.textContent = this.i18n.instant('UI.DATEPICKER.TODAY');
            clear.textContent = this.i18n.instant('UI.DATEPICKER.CLEAR');
        };
        setLabels();
        this.i18n.langChange$.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(setLabels);

        today.addEventListener('click', () =>
            this.zone.run(() => {
                const base = this.mode() === 'datetime' ? new Date() : startOfDay(new Date());
                // 2nd arg true → fires onChange, so the form updates via the CVA.
                instance.setDate(this.mode() === 'range' ? [base, base] : base, true);
            })
        );
        clear.addEventListener('click', () => this.zone.run(() => instance.clear())); // → onChange [] → null

        footer.append(today, clear);
        instance.calendarContainer.appendChild(footer);
    }

    private makeFooterButton(): HTMLButtonElement {
        const btn = this.el.nativeElement.ownerDocument.createElement('button');
        btn.type = 'button';
        btn.className = 'ui-datepicker-footer__btn';
        return btn;
    }
}
