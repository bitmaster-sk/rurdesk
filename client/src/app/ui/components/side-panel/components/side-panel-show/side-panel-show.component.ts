import {
    ChangeDetectionStrategy,
    Component,
    ViewEncapsulation,
    inject,
    input
} from '@angular/core';
import { SidePanelControl } from '../../side-panel.store';

/** Brings a hidden `ui-side-panel` back; sits first in the content header and is there only while the panel is hidden. */
@Component({
    selector: 'ui-side-panel-show',
    standalone: false,
    templateUrl: './side-panel-show.component.html',
    // Styles are global (side-panel-show.styles.scss); no styleUrl by design.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: { class: 'ui-side-panel-show' }
})
export class UiSidePanelShowComponent {
    protected readonly control = inject(SidePanelControl);

    public readonly label = input.required<string>();
}
