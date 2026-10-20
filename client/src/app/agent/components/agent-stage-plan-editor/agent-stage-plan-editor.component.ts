import { ChangeDetectionStrategy, Component, input, output, signal } from '@angular/core';
import { Skill } from 'src/app/shared/model/skill.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import {
    AgentStagePlanEditorLock,
    AgentStagePlanEditorRow,
    AgentStagePlanMaxTurnsChange,
    AgentStagePlanModelPick,
    AgentStagePlanSkillToggle
} from '../../entity/agent-stage-plan-editor.entity';
import { AgentStage, STAGE_LABELS } from '../../model/agent-stage.enum';

@Component({
    selector: 'app-agent-stage-plan-editor',
    templateUrl: './agent-stage-plan-editor.component.html',
    styleUrls: ['./agent-stage-plan-editor.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentStagePlanEditorComponent {
    public readonly rows = input.required<AgentStagePlanEditorRow[]>();
    public readonly skills = input.required<Skill[]>();
    public readonly models = input<string[]>([]);
    public readonly maxTurnsLimit = input<number | null>(null);
    public readonly saveStatusByStage = input<Record<string, UiSaveState>>({});

    public readonly skillToggled = output<AgentStagePlanSkillToggle>();
    public readonly modelPicked = output<AgentStagePlanModelPick>();
    public readonly maxTurnsChanged = output<AgentStagePlanMaxTurnsChange>();

    protected readonly AgentStagePlanEditorLock = AgentStagePlanEditorLock;
    private readonly openStages = signal<Set<AgentStage>>(new Set());

    protected stageLabel(stage: AgentStage): string {
        return STAGE_LABELS[stage];
    }

    protected isOpen(stage: AgentStage): boolean {
        return this.openStages().has(stage);
    }

    protected onToggleOpen(stage: AgentStage): void {
        const next = new Set(this.openStages());
        if (next.has(stage)) {
            next.delete(stage);
        } else {
            next.add(stage);
        }
        this.openStages.set(next);
    }

    protected isChosen(row: AgentStagePlanEditorRow, idSkill: number): boolean {
        return row.idsSkill.includes(idSkill);
    }

    protected chosenSkillNames(row: AgentStagePlanEditorRow): string {
        return this.skills()
            .filter(skill => row.idsSkill.includes(skill.idSkill))
            .map(skill => skill.name)
            .join(', ');
    }

    protected saveStatus(stage: AgentStage): UiSaveState {
        return this.saveStatusByStage()[stage] ?? UiSaveState.Idle;
    }

    protected onToggleSkill(row: AgentStagePlanEditorRow, idSkill: number): void {
        this.skillToggled.emit({ stage: row.stage, idSkill });
    }

    protected onPickModel(row: AgentStagePlanEditorRow, model: string | null): void {
        if (model !== row.model) {
            this.modelPicked.emit({ stage: row.stage, model });
        }
    }

    protected onMaxTurns(row: AgentStagePlanEditorRow, event: Event): void {
        const value = Number((event.target as HTMLInputElement).value);
        const maxTurns = value > 0 ? value : null;
        if (maxTurns !== row.maxTurns) {
            this.maxTurnsChanged.emit({ stage: row.stage, maxTurns });
        }
    }
}
