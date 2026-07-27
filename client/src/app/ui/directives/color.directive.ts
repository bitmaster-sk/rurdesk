import {
    Directive,
    ElementRef,
    HostListener,
    booleanAttribute,
    forwardRef,
    inject,
    input
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { normalizeHexColor } from '../util/ui-color';

/**
 * `ui-color` swatch chrome on a native `<input type="color">`.
 *
 * Custom `ControlValueAccessor` so incoming values are normalised to `#rrggbb`
 * before hitting the element — the native picker silently coerces anything else
 * (incl. `rrggbb` without the `#`) to black. Emitted values are canonical
 * `#rrggbb`. An unparseable stored value shows `fallback` but is NOT written
 * back on load (don't mutate untouched data). Element must stay `type="color"`.
 */
@Directive({
    selector: 'input[uiColor]',
    standalone: false,
    host: {
        'class': 'ui-color',
        'type': 'color',
        '[class.ui-color--invalid]': 'invalid()',
        '[attr.aria-invalid]': 'invalid() || null'
    },
    providers: [
        { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => UiColorDirective), multi: true }
    ]
})
export class UiColorDirective implements ControlValueAccessor {
    private readonly el = inject<ElementRef<HTMLInputElement>>(ElementRef);

    /** Toggles invalid styling. Named `invalid` to mirror ui-input/ui-checkbox. */
    public readonly invalid = input(false, { transform: booleanAttribute });

    /** Colour shown when the stored value can't be parsed as hex. */
    public readonly fallback = input('#000000');

    private onChange: (value: string) => void = () => {};
    private onTouched: () => void = () => {};

    @HostListener('input')
    public onInput(): void {
        // The native element guarantees `#rrggbb` here, so emit it verbatim.
        this.onChange(this.el.nativeElement.value);
    }

    @HostListener('blur')
    public onBlur(): void {
        this.onTouched();
    }

    public writeValue(value: string | null | undefined): void {
        this.el.nativeElement.value = normalizeHexColor(value, this.fallback());
    }

    public registerOnChange(fn: (value: string) => void): void {
        this.onChange = fn;
    }

    public registerOnTouched(fn: () => void): void {
        this.onTouched = fn;
    }

    public setDisabledState(isDisabled: boolean): void {
        this.el.nativeElement.disabled = isDisabled;
    }
}
