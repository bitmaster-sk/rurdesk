import { ChangeDetectionStrategy, Component, ViewEncapsulation, input } from '@angular/core';

/**
 * Inline loading indicator: a spinning Tabler `loader-2` icon.
 * Blocking/overlay loading is intentionally out of scope (a future `ui-mask`
 * wrapper would compose `<ui-loader/>`, not a flag here).
 */
@Component({
    selector: 'ui-loader',
    standalone: false,
    templateUrl: './loader.component.html',
    // Styles are global (see src/app/ui/ui.styles.scss); no styleUrl → no FOUC.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        'role': 'status',
        '[attr.aria-label]': 'ariaLabel()'
    }
})
export class UiLoaderComponent {
    /** Icon size in pixels. */
    public readonly size = input<number>(40);

    /** Accessible label announced by screen readers. */
    public readonly ariaLabel = input<string>('');
}
