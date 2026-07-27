import {
    ChangeDetectionStrategy,
    Component,
    ViewEncapsulation,
    computed,
    input
} from '@angular/core';

/**
 * Animated numeric counter: when `value` changes, each digit rolls vertically
 * to its new position (mechanical-odometer style) instead of snapping.
 *
 * Uses `font-variant-numeric: tabular-nums` so digits are equal-width and the
 * layout doesn't jitter while rolling. Honours `prefers-reduced-motion`.
 */
@Component({
    selector: 'ui-odometer',
    standalone: false,
    templateUrl: './odometer.component.html',
    // Styles are global (src/app/ui/ui.styles.scss); no styleUrl by design.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        'class': 'ui-odometer',
        '[attr.aria-label]': 'safeValue()'
    }
})
export class UiOdometerComponent {
    /** Number rendered by the counter. Negative values clamp to 0. */
    public readonly value = input.required<number>();

    protected readonly numbers = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];

    protected readonly safeValue = computed(() => Math.max(0, Math.trunc(this.value() || 0)));

    protected readonly digits = computed(() => String(this.safeValue()).split('').map(Number));
}
