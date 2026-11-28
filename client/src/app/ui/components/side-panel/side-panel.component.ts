import {
    ChangeDetectionStrategy,
    Component,
    ViewEncapsulation,
    inject,
    input
} from '@angular/core';
import { SidePanelControl } from './side-panel.store';

/**
 * The secondary panel next to the side menu (Wiki, Messages, Agent chat): a header with the title,
 * optional `[uiPanelActions]` and the hide button, an optional `[uiPanelFilter]`, the scrolling
 * body (a loader while `isLoading`) and an optional `[uiPanelFooter]`. Hidden while its `SidePanelControl` says so;
 * `ui-side-panel-show` brings it back.
 */
@Component({
    selector: 'ui-side-panel',
    standalone: false,
    templateUrl: './side-panel.component.html',
    // Styles are global (src/app/ui/ui.styles.scss); no styleUrl by design.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        'class': 'ui-side-panel',
        // A class, not the `hidden` attribute: Tailwind's preflight makes `[hidden]` display: none
        // !important, which a page that must keep the panel in view could not override.
        '[class.ui-side-panel--hidden]': 'control.isHidden()'
    }
})
export class UiSidePanelComponent {
    protected readonly control = inject(SidePanelControl);

    public readonly title = input.required<string>();
    public readonly hideLabel = input.required<string>();
    /** A loader in the middle of the body stands in for the rows until they arrive. */
    public readonly isLoading = input(false);
}
