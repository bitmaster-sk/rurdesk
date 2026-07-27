import { Directive, booleanAttribute, input } from '@angular/core';

/**
 * Applies the product `ui-input` chrome to a native `<input>`. Adds the class
 * additively (so utility classes like `w-full`/`flex-1` survive) and toggles
 * `ui-input--invalid` + `aria-invalid`.
 *
 * Deliberately does NOT provide `NG_VALUE_ACCESSOR` — the native
 * `DefaultValueAccessor` already handles the form value pipeline. Declaring one
 * here would shadow it and break forms.
 */
@Directive({
    selector: 'input[uiInput]',
    standalone: false,
    host: {
        'class': 'ui-input',
        '[class.ui-input--invalid]': 'invalid()',
        '[class.ui-input--sm]': "size() === 'small'",
        '[class.ui-input--lg]': "size() === 'large'",
        '[attr.aria-invalid]': 'invalid() || null'
    }
})
export class UiInputDirective {
    /** Toggles invalid styling. Named `invalid` to mirror the shipped ui-button. */
    public readonly invalid = input(false, { transform: booleanAttribute });

    /** Height tier — mirrors ui-select's `size`. Omit for the 32px default. */
    public readonly size = input<'small' | 'large'>();
}
