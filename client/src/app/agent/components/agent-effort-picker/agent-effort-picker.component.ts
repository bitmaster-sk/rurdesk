import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { AGENT_EFFORTS } from '../../constants/agent-effort.constants';
import { AgentEffortInherited } from '../../entity/agent-effort.entity';
import { AgentEffort } from '../../model/agent-effort.enum';

@Component({
    selector: 'app-agent-effort-picker',
    templateUrl: './agent-effort-picker.component.html',
    styleUrls: ['./agent-effort-picker.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentEffortPickerComponent {
    private static nextId = 0;

    public readonly effort = input<AgentEffort | null>(null);
    public readonly inherited = input<AgentEffortInherited | null>(null);
    public readonly label = input('AGENT_EFFORT.LABEL');
    public readonly size = input<'small' | 'medium'>('small');
    public readonly testId = input('effort');
    public readonly hint = input<string | null>(null);

    public readonly effortPicked = output<AgentEffort | null>();

    protected readonly levels = AGENT_EFFORTS;
    protected readonly labelId = `agent-effort-picker-${AgentEffortPickerComponent.nextId++}`;
    protected readonly labelClass = computed(() =>
        this.size() === 'small' ? 'ui-color-text-muted text-xs' : 'ui-color-text-muted text-sm'
    );
    protected readonly inheritedEffort = computed(() =>
        this.effort() === null ? (this.inherited()?.effort ?? null) : null
    );

    protected onPick(level: AgentEffort | null): void {
        this.effortPicked.emit(level === this.effort() ? null : level);
    }
}
