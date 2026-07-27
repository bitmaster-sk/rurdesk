import { ChangeDetectionStrategy, Component, ViewEncapsulation, input } from '@angular/core';

export type UiTagSeverity =
    'primary' | 'secondary' | 'success' | 'info' | 'warn' | 'danger' | 'contrast';

/**
 * Presentational status pill: a `value` label coloured by `severity`.
 *
 * Colours come from the `--ui-tag-*` tokens. Severity is applied as an additive
 * `.ui-tag--<severity>` host class so external classes (e.g. `ml-1`,
 * `.anchor-outdated-tag`) passed by the call site are preserved.
 */
@Component({
    selector: 'ui-tag',
    standalone: false,
    templateUrl: './tag.component.html',
    // Styles are global (src/app/ui/ui.styles.scss); no styleUrl by design.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        'class': 'ui-tag',
        '[class.ui-tag--primary]': "severity() === 'primary'",
        '[class.ui-tag--secondary]': "severity() === 'secondary'",
        '[class.ui-tag--success]': "severity() === 'success'",
        '[class.ui-tag--info]': "severity() === 'info'",
        '[class.ui-tag--warn]': "severity() === 'warn'",
        '[class.ui-tag--danger]': "severity() === 'danger'",
        '[class.ui-tag--contrast]': "severity() === 'contrast'"
    }
})
export class UiTagComponent {
    /** Text label rendered inside the tag. */
    public readonly value = input<string>();
    /** Colour palette. Defaults to `secondary`. */
    public readonly severity = input<UiTagSeverity>('secondary');
}
