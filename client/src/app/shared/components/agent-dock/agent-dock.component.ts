import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnInit,
    inject,
    input,
    output
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { signal } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { AgentRunApi } from '../../../agent/api/agent-run.api.service';
import { SKILL_STAGES } from '../../../agent/constants/skill-stages.enum';
import { AgentStage, STAGE_LABELS } from '../../../agent/model/agent-stage.enum';
import { AgentRun } from '../../../agent/model/agent-run.model';
import { ShortWorkflow } from '../../../agent/util/short-workflow';
import { ToastNotificationService } from '../../../core/toast-notification.service';
import { ProjectSkillApi } from '../../../project/api/project-skill.api.service';
import { SkillApi } from '../../api/skill.api.service';
import { CreateAgentRunReq } from '../../../agent/model/agent-run.model';
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
    private readonly chosen = signal<Set<string>>(new Set());

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

    protected stageLabel(stage: AgentStage): string {
        return STAGE_LABELS[stage];
    }

    protected isChosen(idSkill: number, stage: AgentStage): boolean {
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
            isShortWorkflow: this.isShortWorkflow()
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
        this.isShortWorkflow.set(ShortWorkflow.isShort(run.stagePlan));
        this.isPrefilledFromRun.set(true);
        this.isLoading.set(false);
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
