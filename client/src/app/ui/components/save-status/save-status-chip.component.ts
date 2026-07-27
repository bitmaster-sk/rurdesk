import { ChangeDetectionStrategy, Component, ViewEncapsulation, input } from '@angular/core';

export enum UiSaveState {
    Idle = 'idle',
    Saving = 'saving',
    Saved = 'saved',
    Error = 'error'
}

/**
 * Presentational auto-save chip rendered inside a CDK overlay by `[uiSaveStatus]`.
 * Not used directly in templates — attached as a `ComponentPortal`. Floats over
 * the field's corner (out of flow) so it never shifts layout.
 */
@Component({
    selector: 'ui-save-status-chip',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    encapsulation: ViewEncapsulation.None,
    template: `
        <div
            class="ui-save-chip"
            [class.ui-save-chip--saved]="state() === SaveState.Saved"
            [class.ui-save-chip--error]="state() === SaveState.Error"
            role="status"
        >
            @switch (state()) {
                @case (SaveState.Saving) {
                    <span class="ui-save-chip__spinner"></span>
                    <span>{{ 'UI.SAVE_STATUS.SAVING' | translate }}</span>
                }
                @case (SaveState.Saved) {
                    <tabler-icon icon="circle-check" [size]="13" aria-hidden="true" />
                    <span>{{ 'UI.SAVE_STATUS.SAVED' | translate }}</span>
                }
                @case (SaveState.Error) {
                    <tabler-icon icon="alert-circle" [size]="13" aria-hidden="true" />
                    <span>{{ 'UI.SAVE_STATUS.FAILED' | translate }}</span>
                }
            }
        </div>
    `
})
export class UiSaveStatusChipComponent {
    public readonly state = input<UiSaveState>(UiSaveState.Idle);
    protected readonly SaveState = UiSaveState;
}
