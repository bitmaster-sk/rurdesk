import { Directive, booleanAttribute, input } from '@angular/core';

/**
 * Applies the product `ui-input` chrome to a native `<textarea>`. Same
 * rationale as {@link UiInputDirective}: additive class, no `NG_VALUE_ACCESSOR`
 * (native accessor owns the value).
 *
 * `.ui-textarea` is only a marker — all visual chrome is inherited from
 * `.ui-input`; height/resize stays the consumer's responsibility.
 */
@Directive({
    selector: 'textarea[uiTextarea]',
    standalone: false,
    host: {
        'class': 'ui-input ui-textarea',
        '[class.ui-input--invalid]': 'invalid()',
        '[attr.aria-invalid]': 'invalid() || null'
    }
})
export class UiTextareaDirective {
    public readonly invalid = input(false, { transform: booleanAttribute });
}
