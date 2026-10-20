import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    computed,
    inject,
    input,
    output
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import omit from 'lodash-es/omit';
import { signal } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { AgentRunApi } from '../../../agent/api/agent-run.api.service';
import { SHORT_WORKFLOW_SKIPPED_STAGES } from '../../../agent/constants/short-workflow-stages.constants';
import { SKILL_STAGES } from '../../../agent/constants/skill-stages.enum';
import {
    AgentStagePlanEditorLock,
    AgentStagePlanEditorRow
} from '../../../agent/entity/agent-stage-plan-editor.entity';
import { AgentStage } from '../../../agent/model/agent-stage.enum';
import { AgentRun } from '../../../agent/model/agent-run.model';
import { ShortWorkflow } from '../../../agent/util/short-workflow';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { ProjectSkillApi } from '../../../project/api/project-skill.api.service';
import { SkillApi } from '../../api/skill.api.service';
import { CreateAgentRunReq, RunChoices } from '../../../agent/model/agent-run.model';
import { AgentOverview } from '../../../agent/model/agent-overview.model';
import { ApiError } from '../../model/api-error.model';
import { Skill } from '../../model/skill.model';

export enum AgentDockKind {
    Skills = 'skills',
    Info = 'info'
}

@Component({
    selector: 'app-agent-dock',
    templateUrl: './agent-dock.component.html',
    styleUrls: ['./agent-dock.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentDockComponent implements OnInit {
    public readonly agent = input.required<User>();
    public readonly kind = input.required<AgentDockKind>();
    public readonly idProject = input.required<number>();
    public readonly idIssuePublic = input.required<number>();
    public readonly overview = input<AgentOverview | null>(null);
    public readonly close = input.required<() => void>();

    public readonly assigned = output<AgentRun>();

    private readonly skillApi = inject(SkillApi);
    private readonly projectSkillApi = inject(ProjectSkillApi);
    private readonly agentRunApi = inject(AgentRunApi);
    private readonly toast = inject(ToastNotificationService);
    private readonly destroyRef = inject(DestroyRef);

    protected readonly stages = SKILL_STAGES;
    protected readonly skills = signal<Skill[]>([]);
    protected readonly isLoading = signal(false);
    protected readonly isAssigning = signal(false);
    protected readonly isShortWorkflow = signal(false);
    protected readonly isPrefilledFromRun = signal(false);
    protected readonly models = computed(() => this.overview()?.models ?? []);
    protected readonly modelDefault = computed(() => this.overview()?.modelDefault ?? null);
    protected readonly runModelValue = computed(() => this.runModel() ?? this.modelDefault());
    private readonly chosen = signal<Set<string>>(new Set());
    private readonly runModel = signal<string | null>(null);
    private readonly modelByStage = signal<Record<string, string>>({});
    private readonly maxTurnsByStage = signal<Record<string, number>>({});
    protected readonly stageRows = computed<AgentStagePlanEditorRow[]>(() =>
        this.stages.map(stage => ({
            stage,
            idsSkill: this.skills()
                .filter(skill => this.isChosen(skill.idSkill, stage))
                .map(skill => skill.idSkill),
            model: this.stageModel(stage),
            maxTurns: this.stageTurns(stage),
            defaultMaxTurns: this.defaultTurns(stage),
            lock: this.isSkipped(stage) ? AgentStagePlanEditorLock.Skipped : null
        }))
    );

    public ngOnInit(): void {
        if (this.kind() !== AgentDockKind.Skills) {
            return;
        }
        this.isLoading.set(true);
        this.skillApi
            .load$()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: all => {
                    this.skills.set(all);
                    this.loadPrefill();
                },
                error: () => {
                    this.isLoading.set(false);
                    this.toast.showError('AGENT.SKILLS.LOAD_ERROR');
                }
            });
    }

    private isChosen(idSkill: number, stage: AgentStage): boolean {
        return this.chosen().has(this.chipKey(idSkill, stage));
    }

    protected onToggle(idSkill: number, stage: AgentStage): void {
        const key = this.chipKey(idSkill, stage);
        const next = new Set(this.chosen());
        if (next.has(key)) {
            next.delete(key);
        } else {
            next.add(key);
        }
        this.chosen.set(next);
    }

    private isSkipped(stage: AgentStage): boolean {
        return this.isShortWorkflow() && SHORT_WORKFLOW_SKIPPED_STAGES.includes(stage);
    }

    protected onPickRunModel(model: string | null): void {
        this.runModel.set(model);
    }

    private stageModel(stage: AgentStage): string | null {
        return this.modelByStage()[stage] ?? null;
    }

    protected onPickStageModel(stage: AgentStage, model: string | null): void {
        const rest = omit(this.modelByStage(), stage);
        this.modelByStage.set(model === null ? rest : { ...rest, [stage]: model });
    }

    private stageTurns(stage: AgentStage): number | null {
        return this.maxTurnsByStage()[stage] ?? null;
    }

    private defaultTurns(stage: AgentStage): number | null {
        const overview = this.overview();
        if (!overview) {
            return null;
        }
        return stage === AgentStage.Implementation
            ? overview.maxTurnsImplement
            : overview.maxTurnsPlan;
    }

    protected onStageTurns(stage: AgentStage, maxTurns: number | null): void {
        const rest = omit(this.maxTurnsByStage(), stage);
        this.maxTurnsByStage.set(maxTurns === null ? rest : { ...rest, [stage]: maxTurns });
    }

    protected onToggleShortWorkflow(event: Event): void {
        this.isShortWorkflow.set((event.target as HTMLInputElement).checked);
    }

    protected onCancel(): void {
        this.close()();
    }

    public onAssign(): void {
        if (this.isLoading() || this.isAssigning()) {
            return;
        }
        const body: CreateAgentRunReq = {
            idUserAgent: this.agent().idUser,
            idsSkillByStage: this.idsSkillByStage(),
            isShortWorkflow: this.isShortWorkflow(),
            ...this.modelChoices()
        };
        this.isAssigning.set(true);
        this.agentRunApi.assignAgent$(this.idProject(), this.idIssuePublic(), body).subscribe({
            next: run => {
                this.isAssigning.set(false);
                this.assigned.emit(run);
                this.close()();
            },
            error: (err: unknown) => {
                this.isAssigning.set(false);
                if (!ApiError.translateKeyOf(err)) {
                    this.toast.showError('AGENT_DOCK.ASSIGN_ERROR');
                }
            }
        });
    }

    protected formatTokens(tokens: number): string {
        if (tokens < 1000) {
            return `${tokens}`;
        }
        if (tokens < 1_000_000) {
            return `${Math.round(tokens / 100) / 10}k`;
        }
        return `${Math.round(tokens / 100_000) / 10}M`;
    }

    protected formatDuration(ms: number | null): string {
        if (ms === null) {
            return '—';
        }
        const minutes = Math.round(ms / 60_000);
        return minutes < 1 ? '< 1 min' : `${minutes} min`;
    }

    private chipKey(idSkill: number, stage: string): string {
        return `${idSkill}:${stage}`;
    }

    private idsSkillByStage(): Record<string, number[]> {
        const byStage: Record<string, number[]> = {};
        this.chosen().forEach(key => {
            const [id, stage] = key.split(':');
            byStage[stage] = [...(byStage[stage] ?? []), Number(id)];
        });
        return byStage;
    }

    private modelChoices(): Partial<RunChoices> {
        const choices: Partial<RunChoices> = {};
        const runModel = this.runModel();
        if (runModel !== null) {
            choices.model = runModel;
        }
        if (Object.keys(this.modelByStage()).length > 0) {
            choices.modelByStage = this.modelByStage();
        }
        if (Object.keys(this.maxTurnsByStage()).length > 0) {
            choices.maxTurnsByStage = this.maxTurnsByStage();
        }
        return choices;
    }

    private loadPrefill(): void {
        this.agentRunApi
            .loadByIssue$(this.idProject(), this.idIssuePublic())
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: run => (run?.isParked ? this.prefillFromRun(run) : this.loadDefaults()),
                error: () => this.loadDefaults()
            });
    }

    private prefillFromRun(run: AgentRun): void {
        const chosen = new Set<string>();
        (run.stagePlan?.stages ?? []).forEach(entry =>
            (entry.idsSkill ?? []).forEach(idSkill => chosen.add(this.chipKey(idSkill, entry.name)))
        );
        this.chosen.set(chosen);
        this.prefillModelChoices(run);
        this.isShortWorkflow.set(ShortWorkflow.isShort(run.stagePlan));
        this.isPrefilledFromRun.set(true);
        this.isLoading.set(false);
    }

    private prefillModelChoices(run: AgentRun): void {
        const modelByStage: Record<string, string> = {};
        const maxTurnsByStage: Record<string, number> = {};
        const isOffered = (model: string | undefined): model is string =>
            !!model && this.models().includes(model);
        (run.stagePlan?.stages ?? []).forEach(entry => {
            if (isOffered(entry.model)) {
                modelByStage[entry.name] = entry.model;
            }
            if (entry.maxTurns) {
                maxTurnsByStage[entry.name] = entry.maxTurns;
            }
        });
        this.runModel.set(isOffered(run.stagePlan?.model) ? run.stagePlan.model : null);
        this.modelByStage.set(modelByStage);
        this.maxTurnsByStage.set(maxTurnsByStage);
    }

    private loadDefaults(): void {
        this.projectSkillApi
            .load$(this.idProject())
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: entries => {
                    this.chosen.set(
                        new Set(entries.map(entry => this.chipKey(entry.idSkill, entry.stage)))
                    );
                    this.isLoading.set(false);
                },
                error: () => {
                    this.isLoading.set(false);
                    this.toast.showError('AGENT.SKILLS.LOAD_ERROR');
                }
            });
    }
}
