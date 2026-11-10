import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { AGENT_EFFORT_LABELS } from '../../constants/agent-effort.constants';
import { AgentEffortConverter } from '../../converter/agent-effort.converter';
import { AgentEffortInherited } from '../../entity/agent-effort.entity';

@Component({
    selector: 'app-agent-effort-hint',
    templateUrl: './agent-effort-hint.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentEffortHintComponent {
    public readonly inherited = input.required<AgentEffortInherited>();

    protected readonly hintKey = computed(() => AgentEffortConverter.toHintKey(this.inherited()));
    protected readonly labels = AGENT_EFFORT_LABELS;
}
