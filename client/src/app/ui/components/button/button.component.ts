import {
    ChangeDetectionStrategy,
    Component,
    ViewEncapsulation,
    booleanAttribute,
    input
} from '@angular/core';

/** Thin wrapper over a native `<button>` applying the product `.ui-button` chrome. */
@Component({
    selector: 'ui-button',
    standalone: false,
    templateUrl: './button.component.html',
    // Styles live in global styles.scss (.ui-button*) — NOT here — so they load
    // upfront and don't FOUC (ViewEncapsulation.None injects component styles
    // only on first instantiation).
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        '[class.ui-button--fluid]': 'fluid()'
    }
})
export class UiButtonComponent {
    public readonly severity = input<
        'primary' | 'secondary' | 'danger' | 'success' | 'info' | 'warn'
    >('primary');
    public readonly variant = input<'filled' | 'outlined' | 'text'>('filled');
    public readonly size = input<'default' | 'small'>('default');
    /** Optional text label; alternatively project text content. */
    public readonly label = input<string>();
    /** Accessible name — bound to the inner button (required for icon-only). */
    public readonly ariaLabel = input<string>();
    /** Square icon-only shape. */
    public readonly isIconOnly = input<boolean>(false);
    /** Fully rounded shape. */
    public readonly isRounded = input<boolean>(false);
    /** Stretch the button to the full width of its container. */
    public readonly fluid = input(false, { transform: booleanAttribute });
    public readonly loading = input<boolean>(false);
    public readonly disabled = input<boolean>(false);
    public readonly type = input<'button' | 'submit'>('button');
}
