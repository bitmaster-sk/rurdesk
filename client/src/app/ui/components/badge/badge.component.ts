import { ChangeDetectionStrategy, Component, ViewEncapsulation, input } from '@angular/core';

export type UiBadgeSeverity =
    'primary' | 'secondary' | 'success' | 'info' | 'warn' | 'danger' | 'contrast';

/**
 * Small inline count pill: a `value` coloured by `severity` (solid fill).
 *
 * Colours come from the `--ui-badge-*` tokens. Severity is applied as an
 * additive `.ui-badge--<severity>` host class so external classes passed by the
 * call site are preserved.
 */
@Component({
    selector: 'ui-badge',
    standalone: false,
    templateUrl: './badge.component.html',
    // Styles are global (src/app/ui/ui.styles.scss); no styleUrl by design.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        'class': 'ui-badge',
        '[class.ui-badge--primary]': "severity() === 'primary'",
        '[class.ui-badge--secondary]': "severity() === 'secondary'",
        '[class.ui-badge--success]': "severity() === 'success'",
        '[class.ui-badge--info]': "severity() === 'info'",
        '[class.ui-badge--warn]': "severity() === 'warn'",
        '[class.ui-badge--danger]': "severity() === 'danger'",
        '[class.ui-badge--contrast]': "severity() === 'contrast'"
    }
})
export class UiBadgeComponent {
    /** Text/number rendered inside the badge. */
    public readonly value = input<string | number>();
    /** Colour palette. Defaults to `secondary` (matches Aura's default). */
    public readonly severity = input<UiBadgeSeverity>('secondary');
}
