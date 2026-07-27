import {
    ChangeDetectionStrategy,
    Component,
    ViewEncapsulation,
    booleanAttribute,
    computed,
    forwardRef,
    input,
    signal
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

/**
 * Boolean toggle rendered as a button (pressed / unpressed). Real
 * `ControlValueAccessor` over a boolean value.
 *
 * Reuses the shipped `.ui-button` system: unpressed = neutral `outlined
 * secondary`, pressed = the `severity` fill (default `primary`). No new tokens.
 * Label/icon are projected as content. `aria-pressed` conveys the toggle state.
 */
export type UiToggleButtonSeverity =
    'primary' | 'secondary' | 'danger' | 'success' | 'info' | 'warn';

@Component({
    selector: 'ui-toggle-button',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UiToggleButtonComponent),
            multi: true
        }
    ],
    host: { class: 'ui-toggle-button' },
    template: `
        <button
            type="button"
            class="ui-button"
            [class]="btnClasses()"
            [attr.aria-pressed]="pressed()"
            [disabled]="isDisabled()"
            (click)="onToggle()"
        >
            <ng-content></ng-content>
        </button>
    `
})
export class UiToggleButtonComponent implements ControlValueAccessor {
    public readonly size = input<'default' | 'small'>('default');
    /** Colour of the pressed (active) state. Unpressed is always neutral. */
    public readonly severity = input<UiToggleButtonSeverity>('primary');
    public readonly disabled = input(false, { transform: booleanAttribute });

    protected readonly pressed = signal(false);

    /** Pressed = severity fill; unpressed = neutral outlined secondary. */
    protected readonly btnClasses = computed(() => ({
        'ui-button--small': this.size() === 'small',
        'ui-button--outlined': !this.pressed(),
        'ui-button--secondary': !this.pressed(),
        ['ui-button--' + this.severity()]: this.pressed()
    }));
    private readonly cvaDisabled = signal(false);
    protected readonly isDisabled = computed(() => this.disabled() || this.cvaDisabled());

    private onChange: (value: boolean) => void = () => {};
    private onTouched: () => void = () => {};

    protected onToggle(): void {
        const next = !this.pressed();
        this.pressed.set(next);
        this.onChange(next);
        this.onTouched();
    }

    // ── ControlValueAccessor ──────────────────────────────────────────────
    public writeValue(value: boolean): void {
        this.pressed.set(!!value);
    }

    public registerOnChange(fn: (value: boolean) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }

    public setDisabledState(isDisabled: boolean): void {
        this.cvaDisabled.set(isDisabled);
    }
}
