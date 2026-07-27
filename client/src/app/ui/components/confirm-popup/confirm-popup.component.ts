import {
    ChangeDetectionStrategy,
    Component,
    ViewEncapsulation,
    input,
    output
} from '@angular/core';

/**
 * Presentational confirmation panel. Rendered into a CDK overlay by
 * {@link UiConfirmDirective}; it does not position itself.
 *
 * Defaults come from the `UI.CONFIRMATION` i18n section so call sites need no
 * per-usage translations.
 */
@Component({
    selector: 'ui-confirm-popup',
    standalone: false,
    templateUrl: './confirm-popup.component.html',
    // Styles are global (see src/app/ui/ui.styles.scss); no styleUrl → no FOUC.
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None
})
export class UiConfirmPopupComponent {
    /** Message to confirm. Falls back to `UI.CONFIRMATION.MESSAGE` when empty. */
    public readonly text = input<string>();
    /** Accept button label. Falls back to `UI.CONFIRMATION.YES` when empty. */
    public readonly acceptLabel = input<string>();
    /** Reject button label. Falls back to `UI.CONFIRMATION.NO` when empty. */
    public readonly rejectLabel = input<string>();

    public readonly accepted = output<void>();
    public readonly rejected = output<void>();

    protected onAccept(): void {
        this.accepted.emit();
    }

    protected onReject(): void {
        this.rejected.emit();
    }
}
