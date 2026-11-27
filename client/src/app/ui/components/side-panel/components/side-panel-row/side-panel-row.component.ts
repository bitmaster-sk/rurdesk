import { ChangeDetectionStrategy, Component, ViewEncapsulation, input } from '@angular/core';

/**
 * The one row of `ui-side-panel`: an optional `[uiRowLead]` (the wiki chevron), the icon, a label
 * clamped to two lines and named in full on hover, and an optional `[uiRowTrail]` (count, dot,
 * date or mark).
 */
@Component({
    // An attribute component, like Angular Material's `button[mat-button]`: the row stays the
    // consumer's own <a> or <button>, with its routerLink, click and keyboard semantics.
    // eslint-disable-next-line @angular-eslint/component-selector
    selector: 'a[uiSidePanelRow], button[uiSidePanelRow]',
    standalone: false,
    templateUrl: './side-panel-row.component.html',
    // Styles are global (side-panel-row.styles.scss); no styleUrl by design.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    host: {
        'class': 'ui-side-panel-row',
        '[class.ui-side-panel-row--active]': 'isActive()',
        '[attr.aria-current]': "isActive() ? 'page' : null",
        '[attr.title]': 'label()'
    }
})
export class UiSidePanelRowComponent {
    public readonly icon = input.required<string>();
    public readonly label = input.required<string>();
    public readonly isActive = input(false);
}
