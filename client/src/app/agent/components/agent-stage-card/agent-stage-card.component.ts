import {
    ChangeDetectionStrategy,
    Component,
    computed,
    input,
    linkedSignal,
    output
} from '@angular/core';
import { Skill } from 'src/app/shared/model/skill.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { AgentStageCardBody, AgentStageCardView } from '../../entity/agent-stage-card.entity';
import {
    AgentStagePlanEditorRow,
    AgentStagePlanMaxTurnsChange,
    AgentStagePlanModelPick,
    AgentStagePlanSkillToggle
} from '../../entity/agent-stage-plan-editor.entity';
import { RunWikiStage } from '../../entity/run-wiki-context.entity';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';

@Component({
    selector: 'app-agent-stage-card',
    templateUrl: './agent-stage-card.component.html',
    styleUrls: ['./agent-stage-card.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentStageCardComponent {
    public readonly view = input.required<AgentStageCardView>();
    public readonly planRow = input<AgentStagePlanEditorRow | null>(null);
    public readonly wikiStage = input<RunWikiStage | null>(null);
    public readonly skills = input<Skill[]>([]);
    public readonly models = input<string[]>([]);
    public readonly maxTurnsLimit = input<number | null>(null);
    public readonly saveState = input<UiSaveState>(UiSaveState.Idle);

    public readonly skillToggled = output<AgentStagePlanSkillToggle>();
    public readonly modelPicked = output<AgentStagePlanModelPick>();
    public readonly maxTurnsChanged = output<AgentStagePlanMaxTurnsChange>();

    protected readonly AgentStageCardBody = AgentStageCardBody;

    private readonly isOpenByDefault = computed(() => this.view().isOpenByDefault);
    protected readonly isOpen = linkedSignal(() => this.isOpenByDefault());

    protected readonly canOpen = computed(() => this.view().body !== AgentStageCardBody.None);

    protected readonly duration = computed(() => {
        const durationMs = this.view().durationMs;
        return durationMs > 0 ? DurationFormatter.millisecondsToString(durationMs) : null;
    });

    protected onToggle(): void {
        if (this.canOpen()) {
            this.isOpen.update(isOpen => !isOpen);
        }
    }

    protected pluralKey(key: string, count: number): string {
        return `${key}.${count === 1 ? 'SINGULAR' : 'PLURAL'}`;
    }
}
