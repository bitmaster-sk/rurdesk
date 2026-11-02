import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { Skill } from 'src/app/shared/model/skill.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import {
    AgentStagePlanEditorRow,
    AgentStagePlanMaxTurnsChange,
    AgentStagePlanModelPick,
    AgentStagePlanSkillToggle
} from '../../entity/agent-stage-plan-editor.entity';

@Component({
    selector: 'app-agent-stage-plan-form',
    templateUrl: './agent-stage-plan-form.component.html',
    styleUrls: ['./agent-stage-plan-form.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentStagePlanFormComponent {
    public readonly row = input.required<AgentStagePlanEditorRow>();
    public readonly skills = input.required<Skill[]>();
    public readonly models = input<string[]>([]);
    public readonly maxTurnsLimit = input<number | null>(null);
    public readonly saveState = input<UiSaveState>(UiSaveState.Idle);
    public readonly isDefaultsHintShown = input(false);
    public readonly size = input<'small' | 'medium'>('small');
    public readonly modelPlaceholder = input('AGENT_STAGE_PLAN_EDITOR.MODEL_INHERIT');

    public readonly skillToggled = output<AgentStagePlanSkillToggle>();
    public readonly modelPicked = output<AgentStagePlanModelPick>();
    public readonly maxTurnsChanged = output<AgentStagePlanMaxTurnsChange>();

    protected readonly controlSize = computed(() =>
        this.size() === 'small' ? 'small' : undefined
    );
    protected readonly labelClass = computed(() =>
        this.size() === 'small' ? 'ui-color-text-muted text-xs' : 'ui-color-text-muted text-sm'
    );

    protected isChosen(idSkill: number): boolean {
        return this.row().idsSkill.includes(idSkill);
    }

    protected onToggleSkill(idSkill: number): void {
        this.skillToggled.emit({ stage: this.row().stage, idSkill });
    }

    protected onPickModel(model: string | null): void {
        if (model !== this.row().model) {
            this.modelPicked.emit({ stage: this.row().stage, model });
        }
    }

    protected onMaxTurns(event: Event): void {
        const value = Number((event.target as HTMLInputElement).value);
        const maxTurns = value > 0 ? value : null;
        if (maxTurns !== this.row().maxTurns) {
            this.maxTurnsChanged.emit({ stage: this.row().stage, maxTurns });
        }
    }
}
