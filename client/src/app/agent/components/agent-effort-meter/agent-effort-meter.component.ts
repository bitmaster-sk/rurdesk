import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { AGENT_EFFORTS } from '../../constants/agent-effort.constants';
import { AgentEffort } from '../../model/agent-effort.enum';

@Component({
    selector: 'app-agent-effort-meter',
    templateUrl: './agent-effort-meter.component.html',
    styleUrls: ['./agent-effort-meter.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentEffortMeterComponent {
    public readonly effort = input.required<AgentEffort>();

    protected readonly bars = AGENT_EFFORTS.map((_, index) => index + 1);
    protected readonly level = computed(() => AGENT_EFFORTS.indexOf(this.effort()) + 1);
}
