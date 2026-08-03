import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    ViewEncapsulation,
    booleanAttribute,
    computed,
    forwardRef,
    inject,
    input,
    signal,
    viewChild,
    viewChildren
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ControlValueAccessor, FormControl, NG_VALUE_ACCESSOR } from '@angular/forms';
import { TranslateService } from '@ngx-translate/core';
import { UI_DATE_PATTERN, uiFormatDate } from '../../util/ui-date-format';
import { UiPopoverComponent } from '../popover/popover.component';

/** One row in the preset list. `value` is opaque here — the consumer defines it. */
export interface UiDateRangePreset {
    label: string;
    value: string;
}

/** Either a rolling window or a fixed range, never both. */
export interface UiDateRangeValue {
    /** Consumer-defined rolling-window token, e.g. a duration like '30d'. */
    preset?: string;
    from?: Date;
    to?: Date;
}

let nextPanelId = 0;

@Component({
    selector: 'ui-date-range-select',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UiDateRangeSelectComponent),
            multi: true
        }
    ],
    templateUrl: './date-range-select.component.html'
})
export class UiDateRangeSelectComponent implements ControlValueAccessor {
    private readonly i18n = inject(TranslateService);

    /** "Custom range" is appended by the component itself. */
    public readonly presets = input<readonly UiDateRangePreset[]>([]);
    public readonly placeholder = input<string>('');
    public readonly inputId = input<string>();
    public readonly size = input<'small' | undefined>();
    public readonly showClear = input(true, { transform: booleanAttribute });
    public readonly disabled = input(false, { transform: booleanAttribute });

    protected readonly panelId = `ui-date-range-panel-${nextPanelId++}`;

    protected readonly value = signal<UiDateRangeValue | null>(null);
    protected readonly isOpen = signal(false);
    /** Sticky so the calendar survives picking the second date. */
    protected readonly isCustomOpen = signal(false);

    /** Set only for an intentional dismissal — an outside click must not steal focus. */
    protected restoreFocusOnHide = false;

    private readonly formsDisabled = signal(false);
    protected readonly isDisabled = computed(() => this.disabled() || this.formsDisabled());

    /** A real control, not a signal, because the child datepicker is a CVA. */
    protected readonly rangeControl = new FormControl<Date[] | null>(null);

    private readonly panel = viewChild.required<UiPopoverComponent>('panel');
    private readonly triggerEl = viewChild.required<ElementRef<HTMLElement>>('trigger');
    private readonly presetButtons = viewChildren<ElementRef<HTMLButtonElement>>('presetBtn');

    protected readonly isCustom = computed(
        () => this.isCustomOpen() || (!this.value()?.preset && !!this.value())
    );

    protected readonly customLabel = this.i18n.instant('UI.DATE_RANGE.CUSTOM');

    /** Preset label when one is chosen, formatted dates otherwise. */
    protected readonly triggerLabel = computed(() => {
        const current = this.value();
        if (!current) {
            return '';
        }
        if (current.preset) {
            return (
                this.presets().find(preset => preset.value === current.preset)?.label ??
                current.preset
            );
        }
        const from = current.from ? uiFormatDate(current.from, UI_DATE_PATTERN) : '';
        const to = current.to ? uiFormatDate(current.to, UI_DATE_PATTERN) : '';
        return from && to ? `${from} – ${to}` : from || to;
    });

    private onChange: (value: UiDateRangeValue | null) => void = () => {};
    protected onTouched: () => void = () => {};

    public constructor() {
        this.rangeControl.valueChanges
            .pipe(takeUntilDestroyed())
            .subscribe(range => this.onRangeChange(range));
    }

    // ── ControlValueAccessor ────────────────────────────────────────────────

    public writeValue(value: UiDateRangeValue | null): void {
        this.applyValue(value ?? null);
    }

    public registerOnChange(fn: (value: UiDateRangeValue | null) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }

    public setDisabledState(isDisabled: boolean): void {
        this.formsDisabled.set(isDisabled);
        if (isDisabled) {
            this.rangeControl.disable({ emitEvent: false });
            if (this.isOpen()) {
                this.panel().hide();
            }
        } else {
            this.rangeControl.enable({ emitEvent: false });
        }
    }

    // ── interaction ─────────────────────────────────────────────────────────

    protected onTriggerClick(event: Event): void {
        if (this.isDisabled()) {
            return;
        }
        this.isOpen.set(!this.isOpen());
        this.panel().toggle(event);
    }

    protected onTriggerKeydown(event: KeyboardEvent): void {
        const isArrow = event.key === 'ArrowDown' || event.key === 'ArrowUp';
        if (!isArrow && event.key !== 'Enter' && event.key !== ' ') {
            return;
        }
        event.preventDefault();
        // The trigger sits outside the overlay, so without this the global list
        // hotkeys would move the task selection behind the panel as well.
        event.stopPropagation();

        if (this.isOpen() && !isArrow) {
            this.closePanel();
            return;
        }
        if (!this.isOpen()) {
            this.onTriggerClick(event);
        }
        this.focusPreset(event.key === 'ArrowUp' ? -1 : 0);
    }

    /** Roving focus over the preset rows — the panel is a body-level overlay. */
    protected onPanelKeydown(event: KeyboardEvent): void {
        if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') {
            return;
        }
        const buttons = this.presetButtons();
        const current = buttons.findIndex(
            button => button.nativeElement === document.activeElement
        );
        if (current === -1) {
            return;
        }
        event.preventDefault();
        event.stopPropagation();
        this.focusPreset(current + (event.key === 'ArrowDown' ? 1 : -1));
    }

    protected onPanelHide(): void {
        this.isOpen.set(false);
        this.onTouched();
        if (this.restoreFocusOnHide) {
            this.triggerEl().nativeElement.focus();
        }
        this.restoreFocusOnHide = false;
    }

    protected onPickPreset(preset: UiDateRangePreset): void {
        if (this.isDisabled()) {
            return;
        }
        this.isCustomOpen.set(false);
        this.propagate({ preset: preset.value });
        this.closePanel();
    }

    /** Reveals the calendar; the value changes only once a full range is picked. */
    protected onPickCustom(): void {
        if (this.isDisabled()) {
            return;
        }
        this.isCustomOpen.set(true);
        // Re-anchor once flatpickr's lazily imported DOM lands.
        setTimeout(() => this.panel().reposition());
    }

    protected onClear(event: Event): void {
        event.stopPropagation();
        this.isCustomOpen.set(false);
        this.propagate(null);
        this.onTouched();
    }

    private onRangeChange(range: Date[] | null): void {
        if (this.isDisabled()) {
            return;
        }
        // null can only be the footer Clear — applyValue writes with emitEvent: false.
        if (range === null) {
            this.propagate(null);
            this.isCustomOpen.set(true); // keep the calendar available for a new pick
            return;
        }
        if (range.length < 2) {
            return; // half-finished pick
        }
        this.propagate({ from: range[0], to: range[1] });
        this.closePanel();
    }

    private closePanel(): void {
        this.restoreFocusOnHide = true;
        this.panel().hide();
    }

    private propagate(value: UiDateRangeValue | null): void {
        this.applyValue(value);
        this.onChange(value);
    }

    /** Single place keeping `value`, the calendar and custom mode in step. */
    private applyValue(value: UiDateRangeValue | null): void {
        this.value.set(value);
        this.isCustomOpen.set(!!value && !value.preset && (!!value.from || !!value.to));
        const range = value?.from || value?.to ? [value.from, value.to].filter(Boolean) : null;
        this.rangeControl.setValue(range, { emitEvent: false });
    }

    /** Wraps, so -1 is the last row. Deferred — the rows render after the panel opens. */
    private focusPreset(index: number): void {
        setTimeout(() => {
            const buttons = this.presetButtons();
            if (buttons.length) {
                buttons[(index + buttons.length) % buttons.length].nativeElement.focus();
            }
        });
    }
}
