import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    inject,
    input,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { SKILL_STAGES } from '../../../agent/constants/skill-stages.enum';
import { AgentStage, STAGE_LABELS } from '../../../agent/model/agent-stage.enum';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { SkillApi } from '../../../shared/api/skill.api.service';
import { Skill } from '../../../shared/model/skill.model';
import { UpdateProjectSkillReq } from '../../model/project-skill.model';
import { UiSaveState } from '../../../ui/components/save-status/save-status-chip.component';
import { ProjectSkillApi } from '../../api/project-skill.api.service';
import { Project } from '../../model/project.model';

@Component({
    selector: 'app-agent-skill-matrix',
    templateUrl: './agent-skill-matrix.component.html',
    styleUrls: ['./agent-skill-matrix.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentSkillMatrixComponent implements OnInit {
    public readonly project = input.required<Project>();

    private readonly skillApi = inject(SkillApi);
    private readonly projectSkillApi = inject(ProjectSkillApi);
    private readonly toast = inject(ToastNotificationService);
    private readonly destroyRef = inject(DestroyRef);

    protected readonly stages = SKILL_STAGES;
    protected readonly skills = signal<Skill[]>([]);
    protected readonly isLoading = signal(true);
    private readonly enabled = signal<Set<string>>(new Set());
    private readonly rowStatus = signal<Record<number, UiSaveState>>({});

    public ngOnInit(): void {
        this.skillApi
            .load$()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: all => {
                    this.skills.set(all);
                    this.loadMatrix();
                },
                error: () => {
                    this.isLoading.set(false);
                    this.toast.showError('AGENT.SKILLS.LOAD_ERROR');
                }
            });
    }

    protected stageLabel(stage: AgentStage): string {
        return STAGE_LABELS[stage];
    }

    protected isEnabled(idSkill: number, stage: AgentStage): boolean {
        return this.enabled().has(this.cellKey(idSkill, stage));
    }

    protected rowSaveStatus(idSkill: number): UiSaveState {
        return this.rowStatus()[idSkill] ?? UiSaveState.Idle;
    }

    protected cellSaveStatus(idSkill: number, isLastCell: boolean): UiSaveState {
        return isLastCell ? this.rowSaveStatus(idSkill) : UiSaveState.Idle;
    }

    protected onToggle(idSkill: number, stage: AgentStage): void {
        const key = this.cellKey(idSkill, stage);
        const next = new Set(this.enabled());
        if (next.has(key)) {
            next.delete(key);
        } else {
            next.add(key);
        }
        this.enabled.set(next);
        this.saveRow(idSkill);
    }

    private loadMatrix(): void {
        this.projectSkillApi
            .load$(this.project().idProject)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: entries => {
                    this.enabled.set(
                        new Set(entries.map(entry => this.cellKey(entry.idSkill, entry.stage)))
                    );
                    this.isLoading.set(false);
                },
                error: () => {
                    this.isLoading.set(false);
                    this.toast.showError('AGENT.SKILLS.LOAD_ERROR');
                }
            });
    }

    private saveRow(idSkill: number): void {
        this.setRowStatus(idSkill, UiSaveState.Saving);
        const entries: UpdateProjectSkillReq[] = Array.from(this.enabled()).map(key => {
            const [id, stage] = key.split(':');
            return { idSkill: Number(id), stage: stage as AgentStage };
        });

        this.projectSkillApi
            .replace$(this.project().idProject, entries)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: () => this.setRowStatus(idSkill, UiSaveState.Saved),
                error: () => {
                    this.setRowStatus(idSkill, UiSaveState.Error);
                    this.toast.showError('AGENT.SKILLS.SAVE_ERROR');
                }
            });
    }

    private setRowStatus(idSkill: number, status: UiSaveState): void {
        this.rowStatus.update(all => ({ ...all, [idSkill]: status }));
    }

    private cellKey(idSkill: number, stage: AgentStage): string {
        return `${idSkill}:${stage}`;
    }
}
