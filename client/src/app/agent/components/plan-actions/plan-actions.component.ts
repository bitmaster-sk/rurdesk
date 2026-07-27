import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { AgentRun } from '../../model/agent-run.model';
import { AgentPhase } from '../../model/agent-phase.enum';

@Component({
    selector: 'app-plan-actions',
    templateUrl: './plan-actions.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class PlanActionsComponent {
    public readonly run = input<AgentRun | null>(null);

    public readonly approve = output<void>();

    protected readonly AgentPhase = AgentPhase;

    protected readonly isAwaitingApproval = computed(
        () => this.run()?.phase === AgentPhase.AwaitingApproval
    );

    protected onApprove(): void {
        this.approve.emit();
    }
}
