import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { CiStatus, MrState, MrStatus } from 'src/app/project/model/git-integration.model';

@Component({
    selector: 'app-mr-status-pill',
    templateUrl: './mr-status-pill.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class MrStatusPillComponent {
    public readonly status = input<MrStatus | null>(null);

    protected readonly stateSeverity = computed(() => {
        switch (this.status()?.state) {
            case MrState.Open:
                return 'info';
            case MrState.Merged:
                return 'success';
            case MrState.Closed:
                return 'danger';
            default:
                return 'secondary';
        }
    });

    protected readonly stateLabel = computed(() => {
        switch (this.status()?.state) {
            case MrState.Open:
                return 'MR.STATE.OPEN';
            case MrState.Merged:
                return 'MR.STATE.MERGED';
            case MrState.Closed:
                return 'MR.STATE.CLOSED';
            default:
                return '';
        }
    });

    protected readonly ciSeverity = computed(() => {
        switch (this.status()?.ciStatus) {
            case CiStatus.Success:
                return 'success';
            case CiStatus.Failed:
                return 'danger';
            case CiStatus.Pending:
                return 'warn';
            default:
                return 'secondary';
        }
    });

    protected readonly ciLabel = computed(() => {
        switch (this.status()?.ciStatus) {
            case CiStatus.Success:
                return 'MR.CI.SUCCESS';
            case CiStatus.Failed:
                return 'MR.CI.FAILED';
            case CiStatus.Pending:
                return 'MR.CI.PENDING';
            case CiStatus.Canceled:
                return 'MR.CI.CANCELED';
            case CiStatus.Skipped:
                return 'MR.CI.SKIPPED';
            default:
                return 'MR.CI.UNKNOWN';
        }
    });
}
