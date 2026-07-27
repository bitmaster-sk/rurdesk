import { Directive, booleanAttribute, input } from '@angular/core';

/**
 * Applies the product `ui-checkbox` chrome to a native
 * `<input type="checkbox">`. Adds the class additively (utility classes
 * survive) and toggles
 * `ui-checkbox--invalid` + `aria-invalid`.
 *
 * Deliberately does NOT provide `NG_VALUE_ACCESSOR` — the native
 * `CheckboxControlValueAccessor` already owns the boolean value pipeline
 * (exactly as `[binary]` p-checkbox relied on). Declaring one here would
 * shadow it and break forms. The element MUST stay `type="checkbox"`.
 */
@Directive({
    selector: 'input[uiCheckbox]',
    standalone: false,
    host: {
        'class': 'ui-checkbox',
        '[class.ui-checkbox--invalid]': 'invalid()',
        '[attr.aria-invalid]': 'invalid() || null'
    }
})
export class UiCheckboxDirective {
    /** Toggles invalid styling. Named `invalid` to mirror ui-input/ui-button. */
    public readonly invalid = input(false, { transform: booleanAttribute });
}
