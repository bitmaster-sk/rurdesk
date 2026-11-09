import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { AGENT_EFFORT_LABELS } from '../../constants/agent-effort.constants';
import { AgentEffort } from '../../model/agent-effort.enum';

@Component({
    selector: 'app-agent-effort-label',
    templateUrl: './agent-effort-label.component.html',
    styleUrls: ['./agent-effort-label.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentEffortLabelComponent {
    public readonly effort = input.required<AgentEffort>();

    protected readonly labels = AGENT_EFFORT_LABELS;
}
