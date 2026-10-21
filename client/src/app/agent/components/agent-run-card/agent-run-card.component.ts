import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    effect,
    inject,
    input,
    linkedSignal,
    output,
    signal,
    untracked
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { Skill } from 'src/app/shared/model/skill.model';
import {
    AgentRunStagePlanEntry,
    AgentRunStagePlanRes
} from '../../model/agent-run-stage-plan.model';
import {
    AgentStagePlanEditorLock,
    AgentStagePlanEditorRow,
    AgentStagePlanMaxTurnsChange,
    AgentStagePlanModelPick,
    AgentStagePlanSkillToggle
} from '../../entity/agent-stage-plan-editor.entity';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { AgentRun } from '../../model/agent-run.model';
import {
    AgentPhase,
    PHASE_BADGE_SEVERITY,
    PHASE_LABELS,
    PhaseBadgeSeverity
} from '../../model/agent-phase.enum';
import { AgentStage } from '../../model/agent-stage.enum';
import { User } from 'src/app/auth/model/user.model';
import {
    FailedStageError,
    FailedStageErrorConverter
} from '../../converter/failed-stage-error.converter';
import { ShortWorkflow } from '../../util/short-workflow';
import { AgentStageCardConverter } from '../../converter/agent-stage-card.converter';
import { RunWikiContextConverter } from '../../converter/run-wiki-context.converter';
import {
    AgentStageCardBody,
    AgentStageCardPlanEntry,
    AgentStageCardView
} from '../../entity/agent-stage-card.entity';
import { RunWikiStage } from '../../entity/run-wiki-context.entity';
import { AgentRunStatsStore } from '../../store/agent-run-stats.store';

@Component({
    selector: 'app-agent-run-card',
    templateUrl: './agent-run-card.component.html',
    styleUrls: ['./agent-run-card.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [AgentRunStatsStore]
})
export class AgentRunCardComponent {
    public readonly run = input<AgentRun | null>(null);
    public readonly usersMap = input<Map<number, User>>(new Map());
    public readonly idAssignee = input<number | null>(null);

    public readonly cancelRun = output<void>();
    public readonly continueRun = output<void>();
    public readonly restartRun = output<void>();
    public readonly assignBackRun = output<void>();

    private readonly agentRunApi = inject(AgentRunApi);
    private readonly skillApi = inject(SkillApi);
    private readonly toast = inject(ToastNotificationService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly runStats = inject(AgentRunStatsStore);

    protected readonly runStagePlan = signal<AgentRunStagePlanRes | null>(null);
    protected readonly skillCatalog = signal<Skill[]>([]);
    protected readonly stageStatus = signal<Record<string, UiSaveState>>({});
    protected readonly stats = this.runStats.stats;

    protected readonly isEditable = computed(() => {
        const phase = this.run()?.phase;
        return (
            phase !== undefined &&
            phase !== AgentPhase.Done &&
            phase !== AgentPhase.Failed &&
            phase !== AgentPhase.Cancelled
        );
    });

    protected readonly isShortWorkflow = linkedSignal(() =>
        ShortWorkflow.isShort(this.run()?.stagePlan)
    );
    protected readonly isWorkflowLocked = computed(() =>
        ShortWorkflow.isLocked(this.run()?.stages ?? [])
    );
    protected readonly isSavingWorkflow = signal(false);

    protected readonly AgentPhase = AgentPhase;
    protected readonly phaseSeverity = PHASE_BADGE_SEVERITY;
    protected readonly phaseLabels = PHASE_LABELS;

    private readonly userToggled = signal(false);
    private readonly userOpen = signal(false);

    protected readonly isOpen = computed(() => {
        if (this.userToggled()) {
            return this.userOpen();
        }
        const run = this.run();
        if (!run) return false;
        return (
            run.phase === AgentPhase.AwaitingApproval ||
            run.phase === AgentPhase.AwaitingInput ||
            run.phase === AgentPhase.Failed
        );
    });

    // Drives the avatar pulse. The pulse signals the run is live and the agent
    // may act at any moment. Once a PR is open the agent has finished its work
    // and the run only waits for a human to merge — so pr_open is treated as
    // settled (no pulse), like the terminal phases.
    protected readonly isActive = computed(() => {
        const run = this.run();
        if (!run || run.isParked) return false;
        return (
            run.phase !== AgentPhase.Done &&
            run.phase !== AgentPhase.Failed &&
            run.phase !== AgentPhase.Cancelled &&
            run.phase !== AgentPhase.PrOpen
        );
    });

    protected readonly isWorking = computed(() => {
        const run = this.run();
        return !!run && !run.isParked && run.phase === AgentPhase.InProgress;
    });

    protected readonly isParked = computed(() => !!this.run()?.isParked);

    protected readonly assigneeName = computed(() => {
        const idAssignee = this.idAssignee();
        return idAssignee === null ? null : (this.usersMap().get(idAssignee)?.name ?? null);
    });

    protected readonly parkedReasonKey = computed(() =>
        this.assigneeName() ? 'AGENT.CARD.PARKED_REASSIGNED' : 'AGENT.CARD.PARKED_UNASSIGNED'
    );

    protected readonly isAwaitingInput = computed(
        () => this.run()?.phase === AgentPhase.AwaitingInput
    );

    // Surfaces the failed stage's error for the Failed-phase banner: a stable
    // i18n key derived from the reason code (AGENT.ERROR.<UPPER>) plus the raw
    // provider/agent detail. Null when the run hasn't failed with a reason.
    protected readonly failedStageError = computed<FailedStageError | null>(() =>
        FailedStageErrorConverter.toError(this.run()?.stages ?? [])
    );

    protected readonly runModel = computed(() => this.run()?.stagePlan?.model || null);

    protected readonly stagePlanRows = computed(() => {
        const rows = new Map<string, AgentStagePlanEditorRow>();
        (this.runStagePlan()?.stages ?? []).forEach(entry =>
            rows.set(entry.name, {
                stage: entry.name,
                idsSkill: entry.idsSkill,
                model: entry.model,
                maxTurns: entry.maxTurns,
                defaultMaxTurns: this.defaultMaxTurns(entry.name),
                lock: entry.skip ? AgentStagePlanEditorLock.Skipped : null
            })
        );
        return rows;
    });

    protected readonly stageCards = computed<AgentStageCardView[]>(() => {
        const run = this.run();
        if (!run) {
            return [];
        }
        return AgentStageCardConverter.toViews({
            stages: run.stages ?? [],
            plan: this.cardPlan(run),
            runModel: this.runModel(),
            defaultMaxTurns: stage => this.defaultMaxTurns(stage),
            skillNames: new Map(this.skillCatalog().map(skill => [skill.idSkill, skill.name])),
            agentNames: new Map(
                [...this.usersMap().values()].map(user => [user.idUser, user.name])
            ),
            isEditable: this.isEditable()
        });
    });

    protected readonly firstPlanStage = computed(
        () => this.stageCards().find(card => card.body === AgentStageCardBody.Plan)?.stage ?? null
    );

    protected readonly wikiByStage = computed(() => {
        const run = this.run();
        if (!run) {
            return new Map<string, RunWikiStage>();
        }
        const stages = RunWikiContextConverter.toStages(
            this.runStats.wikiReads(),
            run.idProject,
            (run.stagePlan?.stages ?? []).map(stage => stage.name)
        );
        return new Map(stages.map(stage => [stage.stage, stage]));
    });

    private readonly runStatsKey = computed(() => {
        const run = this.run();
        if (!run || !this.isOpen()) {
            return null;
        }
        const stages = (run.stages ?? [])
            .map(stage => `${stage.stage}:${stage.status}:${stage.attemptNo ?? 0}`)
            .join(',');
        return `${run.idRun}|${run.phase}|${stages}`;
    });

    private readonly openedIdRun = computed(() =>
        this.isOpen() ? (this.run()?.idRun ?? null) : null
    );
    private loadedCatalogIdRun: number | null = null;
    private loadedPlanIdRun: number | null = null;

    public constructor() {
        effect(() => {
            if (this.runStatsKey() === null) {
                return;
            }
            const idRun = untracked(() => this.run()?.idRun);
            if (idRun !== undefined) {
                this.runStats.reload(idRun);
            }
        });

        effect(() => {
            const idRun = this.openedIdRun();
            if (idRun !== null && idRun !== this.loadedCatalogIdRun) {
                this.loadedCatalogIdRun = idRun;
                untracked(() => this.loadSkillCatalog());
            }
        });

        effect(() => {
            const idRun = this.openedIdRun();
            if (!this.isEditable()) {
                this.loadedPlanIdRun = null;
                return;
            }
            if (idRun !== null && idRun !== this.loadedPlanIdRun) {
                this.loadedPlanIdRun = idRun;
                untracked(() => this.loadRunStagePlan(idRun));
            }
        });
    }

    protected get phaseSeverityForRun(): PhaseBadgeSeverity {
        const run = this.run();
        if (run?.isParked) {
            return 'warn';
        }
        return run ? (this.phaseSeverity[run.phase] ?? 'secondary') : 'secondary';
    }

    protected get phaseLabelForRun(): string {
        const run = this.run();
        if (run?.isParked) {
            return 'AGENT.PHASE.PAUSED';
        }
        return run ? (this.phaseLabels[run.phase] ?? run.phase) : '';
    }

    protected onToggle(): void {
        this.userToggled.set(true);
        this.userOpen.set(!this.isOpen());
    }

    protected onCancelClick(event: Event): void {
        event.stopPropagation();
        this.cancelRun.emit();
    }

    protected onContinueClick(event: Event): void {
        event.stopPropagation();
        this.continueRun.emit();
    }

    protected onRestartClick(event: Event): void {
        event.stopPropagation();
        this.restartRun.emit();
    }

    protected onAssignBackClick(event: Event): void {
        event.stopPropagation();
        this.assignBackRun.emit();
    }

    protected onToggleSkill(change: AgentStagePlanSkillToggle): void {
        const entry = this.stagePlanEntry(change.stage);
        if (!entry) {
            return;
        }
        const idsSkill = entry.idsSkill.includes(change.idSkill)
            ? entry.idsSkill.filter(idPlanned => idPlanned !== change.idSkill)
            : [...entry.idsSkill, change.idSkill];
        this.saveStagePlanEntry({ ...entry, idsSkill });
    }

    protected onPickStageModel(change: AgentStagePlanModelPick): void {
        const entry = this.stagePlanEntry(change.stage);
        if (entry) {
            this.saveStagePlanEntry({ ...entry, model: change.model });
        }
    }

    protected onStageMaxTurns(change: AgentStagePlanMaxTurnsChange): void {
        const entry = this.stagePlanEntry(change.stage);
        if (entry) {
            this.saveStagePlanEntry({ ...entry, maxTurns: change.maxTurns });
        }
    }

    protected onToggleShortWorkflow(event: Event): void {
        event.preventDefault();
        const run = this.run();
        if (!run) {
            return;
        }
        const isShortWorkflow = !this.isShortWorkflow();
        this.isShortWorkflow.set(isShortWorkflow);
        this.isSavingWorkflow.set(true);
        this.agentRunApi
            .updateWorkflow$(run.idRun, isShortWorkflow)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: () => {
                    this.isSavingWorkflow.set(false);
                    if (this.runStagePlan()) {
                        this.loadRunStagePlan(run.idRun);
                    }
                },
                error: (err: { status?: number }) => {
                    this.isSavingWorkflow.set(false);
                    this.isShortWorkflow.set(!isShortWorkflow);
                    this.toast.showError(
                        err?.status === 409
                            ? 'AGENT.WORKFLOW.RACE_ERROR'
                            : 'AGENT.WORKFLOW.SAVE_ERROR'
                    );
                }
            });
    }

    private stagePlanEntry(stage: AgentStage): AgentRunStagePlanEntry | undefined {
        return this.runStagePlan()?.stages.find(entry => entry.name === stage);
    }

    private saveStagePlanEntry(entry: AgentRunStagePlanEntry): void {
        const idRun = this.run()?.idRun;
        if (idRun === undefined || entry.dispatched) {
            return;
        }
        this.setStageStatus(entry.name, UiSaveState.Saving);
        this.agentRunApi
            .updateStagePlanEntry$(idRun, entry)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: payload => {
                    this.runStagePlan.set(payload);
                    this.setStageStatus(entry.name, UiSaveState.Saved);
                },
                error: (err: { status?: number }) => {
                    this.setStageStatus(entry.name, UiSaveState.Error);
                    if (err?.status === 409) {
                        // The scheduler dispatched the stage between render and click.
                        this.loadRunStagePlan(idRun);
                        this.toast.showError('AGENT.RUN_STAGE_PLAN.RACE_ERROR');
                        return;
                    }
                    this.toast.showError('AGENT.RUN_STAGE_PLAN.SAVE_ERROR');
                }
            });
    }

    protected stageSaveState(stage: string): UiSaveState {
        return this.stageStatus()[stage] ?? UiSaveState.Idle;
    }

    private loadSkillCatalog(): void {
        this.skillApi
            .load$()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: catalog => this.skillCatalog.set(catalog),
                error: () => this.toast.showError('AGENT.SKILLS.LOAD_ERROR')
            });
    }

    private loadRunStagePlan(idRun: number): void {
        this.agentRunApi
            .loadStagePlan$(idRun)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: payload => this.runStagePlan.set(payload),
                error: () => this.toast.showError('AGENT.RUN_STAGE_PLAN.LOAD_ERROR')
            });
    }

    private cardPlan(run: AgentRun): AgentStageCardPlanEntry[] {
        const loaded = this.runStagePlan()?.stages;
        if (loaded) {
            return loaded;
        }
        return (run.stagePlan?.stages ?? []).map(entry => ({
            name: entry.name,
            idsSkill: entry.idsSkill ?? [],
            model: entry.model || null,
            maxTurns: entry.maxTurns || null,
            dispatched: false
        }));
    }

    private defaultMaxTurns(stage: string): number | null {
        const stagePlan = this.runStagePlan();
        if (!stagePlan) {
            return null;
        }
        return stage === String(AgentStage.Implementation)
            ? stagePlan.maxTurnsImplement
            : stagePlan.maxTurnsPlan;
    }

    private setStageStatus(stage: string, status: UiSaveState): void {
        this.stageStatus.update(all => ({ ...all, [stage]: status }));
    }
}
