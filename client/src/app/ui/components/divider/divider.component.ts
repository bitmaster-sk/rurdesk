import { ChangeDetectionStrategy, Component, ViewEncapsulation, input } from '@angular/core';

/**
 * Presentational separator line.
 *
 * The line colour comes from the product token `--ui-color-border`
 * (defined in styles.scss).
 */
@Component({
    selector: 'ui-divider',
    standalone: false,
    templateUrl: './divider.component.html',
    // Styles are global (see src/app/ui/ui.styles.scss); no styleUrl → no FOUC.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        'role': 'separator',
        '[attr.aria-orientation]': 'layout()',
        '[class.ui-divider--horizontal]': "layout() === 'horizontal'",
        '[class.ui-divider--vertical]': "layout() === 'vertical'"
    }
})
export class UiDividerComponent {
    /** Orientation of the separator. Defaults to `horizontal`. */
    public readonly layout = input<'horizontal' | 'vertical'>('horizontal');
}
