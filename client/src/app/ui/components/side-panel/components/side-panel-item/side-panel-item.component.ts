import { ChangeDetectionStrategy, Component, ViewEncapsulation } from '@angular/core';

/**
 * A panel row with an `[uiRowActions]` button beside it, not inside it, so Tab reaches both. The
 * button takes the row's trailing slot on hover, on keyboard focus and while its menu is open.
 */
@Component({
    selector: 'ui-side-panel-item',
    standalone: false,
    templateUrl: './side-panel-item.component.html',
    // Styles are global (side-panel-item.styles.scss); no styleUrl by design.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: { class: 'ui-side-panel-item' }
})
export class UiSidePanelItemComponent {}
