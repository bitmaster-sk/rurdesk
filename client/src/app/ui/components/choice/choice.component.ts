import {
    ChangeDetectionStrategy,
    Component,
    TemplateRef,
    ViewEncapsulation,
    booleanAttribute,
    computed,
    contentChild,
    forwardRef,
    input,
    signal
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { resolveOptionLabel, resolveOptionValue } from '../../util/option-utils';

/**
 * Single-select segmented control: a row of joined buttons, one selectable at
 * a time. Real `ControlValueAccessor` (single
 * value), scoped to the features this app uses — no multi-select, no filtering.
 *
 * Item content: project `<ng-template #item let-item>` for custom rendering
 * (icons/translated labels); otherwise the resolved option label is shown.
 * Equality is `===` — all call sites bind primitive/string-enum values.
 *
 * Segments reuse the `.ui-button` system (unselected = neutral outlined
 * secondary, selected = the `severity` fill, default `primary`) — same colour
 * language as `ui-toggle-button`.
 */
export type UiChoiceSeverity = 'primary' | 'secondary' | 'danger' | 'success' | 'info' | 'warn';

@Component({
    selector: 'ui-choice',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => UiChoiceComponent),
            multi: true
        }
    ],
    host: {
        class: 'ui-choice',
        role: 'group'
    },
    template: `
        @for (option of options(); track $index) {
            <button
                type="button"
                class="ui-button ui-choice__option"
                [class]="optionClasses(option)"
                [attr.aria-pressed]="isSelected(option)"
                [disabled]="isDisabled()"
                (click)="onSelect(option)"
            >
                @if (itemTpl()) {
                    <ng-container
                        [ngTemplateOutlet]="itemTpl()!"
                        [ngTemplateOutletContext]="{ $implicit: option }"
                    ></ng-container>
                } @else {
                    {{ resolveLabel(option) }}
                }
            </button>
        }
    `
})
export class UiChoiceComponent implements ControlValueAccessor {
    public readonly options = input<readonly unknown[]>([]);
    public readonly optionLabel = input<string>();
    public readonly optionValue = input<string>();
    public readonly size = input<'default' | 'small'>('default');
    /** Colour of the selected segment. Unselected segments are always neutral. */
    public readonly severity = input<UiChoiceSeverity>('primary');
    /** When false, the selected option can't be deselected by re-clicking it. */
    public readonly allowEmpty = input(true, { transform: booleanAttribute });
    public readonly disabled = input(false, { transform: booleanAttribute });

    protected readonly itemTpl = contentChild<TemplateRef<{ $implicit: unknown }>>('item');

    private readonly value = signal<unknown>(null);
    private readonly cvaDisabled = signal(false);
    protected readonly isDisabled = computed(() => this.disabled() || this.cvaDisabled());

    private onChange: (value: unknown) => void = () => {};
    private onTouched: () => void = () => {};

    protected isSelected(option: unknown): boolean {
        return resolveOptionValue(option, this.optionValue()) === this.value();
    }

    /** Per-segment classes: selected = severity fill, unselected = outlined secondary. */
    protected optionClasses(option: unknown): Record<string, boolean> {
        const selected = this.isSelected(option);
        return {
            'ui-button--small': this.size() === 'small',
            'ui-button--outlined': !selected,
            'ui-button--secondary': !selected,
            ['ui-button--' + this.severity()]: selected
        };
    }

    protected resolveLabel(option: unknown): string {
        return resolveOptionLabel(option, this.optionLabel());
    }

    protected onSelect(option: unknown): void {
        const optionVal = resolveOptionValue(option, this.optionValue());
        const isReclick = optionVal === this.value();
        if (isReclick && !this.allowEmpty()) {
            return;
        }
        const next = isReclick && this.allowEmpty() ? null : optionVal;
        this.value.set(next);
        this.onChange(next);
        this.onTouched();
    }

    // ── ControlValueAccessor ──────────────────────────────────────────────
    public writeValue(value: unknown): void {
        this.value.set(value);
    }

    public registerOnChange(fn: (value: unknown) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }

    public setDisabledState(isDisabled: boolean): void {
        this.cvaDisabled.set(isDisabled);
    }
}
