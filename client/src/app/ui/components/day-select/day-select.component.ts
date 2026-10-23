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
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ControlValueAccessor, FormControl, NG_VALUE_ACCESSOR } from '@angular/forms';
import { startOfDay, subDays } from 'date-fns';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { UiDate } from '../../util/ui-date';
import { UiPopoverComponent } from '../popover/popover.component';
import { UiDaySelectFormat, UiDaySelectLabel, UiDaySelectRelative } from './day-select-label';

const SHORT_PATTERN = 'EEE d';
const LONG_PATTERN = 'EEE, MMM d';

let nextPanelId = 0;

@Component({
    selector: 'ui-day-select',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UiDaySelectComponent),
            multi: true
        }
    ],
    templateUrl: './day-select.component.html'
})
export class UiDaySelectComponent implements ControlValueAccessor {
    private readonly i18n = inject(I18nService);

    public readonly maxDate = input<Date | null>(null);
    public readonly format = input<UiDaySelectFormat>(UiDaySelectFormat.Short);
    public readonly inputId = input<string>();
    public readonly ariaLabel = input<string>();
    public readonly disabled = input(false, { transform: booleanAttribute });
    public readonly fluid = input(false, { transform: booleanAttribute });

    protected readonly panelId = `ui-day-select-panel-${nextPanelId++}`;

    protected readonly value = signal<Date | null>(null);
    protected readonly isOpen = signal(false);

    protected restoreFocusOnHide = false;

    private readonly formsDisabled = signal(false);
    protected readonly isDisabled = computed(() => this.disabled() || this.formsDisabled());

    protected readonly dayControl = new FormControl<Date | null>(null);

    private readonly panel = viewChild.required<UiPopoverComponent>('panel');
    private readonly triggerEl = viewChild.required<ElementRef<HTMLButtonElement>>('trigger');

    protected readonly UiDaySelectRelative = UiDaySelectRelative;

    protected readonly relative = computed<UiDaySelectRelative | null>(() => {
        const current = this.value();
        return current ? UiDaySelectLabel.relative(current, new Date()) : null;
    });

    protected readonly isPast = computed(() => {
        const current = this.value();
        return !!current && this.relative() !== UiDaySelectRelative.Today;
    });

    protected readonly triggerLabel = computed<string>(() => {
        const current = this.value();
        if (!current) {
            return this.i18n.instant('UI.DAY_SELECT.TODAY');
        }
        if (this.format() === UiDaySelectFormat.Long) {
            return UiDate.format(current, LONG_PATTERN);
        }
        if (this.relative() === UiDaySelectRelative.Today) {
            return this.i18n.instant('UI.DAY_SELECT.TODAY');
        }
        if (this.relative() === UiDaySelectRelative.Yesterday) {
            return this.i18n.instant('UI.DAY_SELECT.YESTERDAY');
        }
        return UiDate.format(current, SHORT_PATTERN);
    });

    private onChange: (value: Date | null) => void = () => {};
    protected onTouched: () => void = () => {};

    public constructor() {
        this.dayControl.valueChanges
            .pipe(takeUntilDestroyed())
            .subscribe(day => this.onCalendarChange(day));
    }

    public writeValue(value: Date | null): void {
        this.applyValue(value ?? null);
    }

    public registerOnChange(fn: (value: Date | null) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }

    public setDisabledState(isDisabled: boolean): void {
        this.formsDisabled.set(isDisabled);
        if (isDisabled) {
            this.dayControl.disable({ emitEvent: false });
            if (this.isOpen()) {
                this.panel().hide();
            }
        } else {
            this.dayControl.enable({ emitEvent: false });
        }
    }

    protected onTriggerClick(event: Event): void {
        if (this.isDisabled()) {
            return;
        }
        this.isOpen.set(!this.isOpen());
        this.panel().toggle(event);
    }

    protected onPickRelative(relative: UiDaySelectRelative): void {
        if (this.isDisabled()) {
            return;
        }
        const now = new Date();
        const day = relative === UiDaySelectRelative.Today ? now : subDays(now, 1);
        this.propagate(startOfDay(day));
        this.closePanel();
    }

    protected onPanelHide(): void {
        this.isOpen.set(false);
        this.onTouched();
        if (this.restoreFocusOnHide) {
            this.triggerEl().nativeElement.focus();
        }
        this.restoreFocusOnHide = false;
    }

    private onCalendarChange(day: Date | null): void {
        if (this.isDisabled() || !day) {
            return;
        }
        this.propagate(startOfDay(day));
        this.closePanel();
    }

    private closePanel(): void {
        this.restoreFocusOnHide = true;
        this.panel().hide();
    }

    private propagate(value: Date): void {
        this.applyValue(value);
        this.onChange(value);
    }

    private applyValue(value: Date | null): void {
        this.value.set(value);
        this.dayControl.setValue(value, { emitEvent: false });
    }
}
