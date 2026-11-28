import { ChangeDetectionStrategy, Component, ViewEncapsulation, input } from '@angular/core';

/** A labelled group of rows in `ui-side-panel`, with an optional `[uiSectionAction]` button. */
@Component({
    selector: 'ui-side-panel-section',
    standalone: false,
    templateUrl: './side-panel-section.component.html',
    // Styles are global (side-panel-section.styles.scss); no styleUrl by design.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        'class': 'ui-side-panel-section',
        'role': 'group',
        '[attr.aria-label]': 'title()'
    }
})
export class UiSidePanelSectionComponent {
    public readonly title = input.required<string>();
}
