import { Directive, booleanAttribute, input } from '@angular/core';

/**
 * Applies the product `ui-toggle` switch chrome to a native
 * `<input type="checkbox">`.
 *
 * Deliberately does NOT provide `NG_VALUE_ACCESSOR` — the native
 * `CheckboxControlValueAccessor` owns the boolean value pipeline. The element
 * MUST stay `type="checkbox"`.
 *
 * `role="switch"` on a native checkbox: the accessibility layer derives the
 * switch's checked state from the element's native checkedness (HTML-AAM), so
 * no manual `aria-checked` sync is needed — programmatic form writes stay
 * correct without a change listener.
 */
@Directive({
    selector: 'input[uiToggle]',
    standalone: false,
    host: {
        'class': 'ui-toggle',
        'role': 'switch',
        '[class.ui-toggle--invalid]': 'invalid()',
        '[attr.aria-invalid]': 'invalid() || null'
    }
})
export class UiToggleDirective {
    /** Toggles invalid styling. Named `invalid` to mirror ui-input/ui-button. */
    public readonly invalid = input(false, { transform: booleanAttribute });
}
