import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    inject,
    input,
    linkedSignal,
    output,
    signal
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
import { UiPopoverComponent } from 'src/app/ui/components/popover/popover.component';
import { AgentRunApi } from '../../api/agent-run.api.service';
import { AgentRun, AgentStageProgress, AgentStageStatus } from '../../model/agent-run.model';
import {
    AgentPhase,
    PHASE_BADGE_SEVERITY,
    PHASE_LABELS,
    PhaseBadgeSeverity
} from '../../model/agent-phase.enum';
import { AgentStage, STAGE_LABELS } from '../../model/agent-stage.enum';
import { User } from 'src/app/auth/model/user.model';
import { HostType } from 'src/app/project/model/git-integration.model';
import { GitHostTerminology } from 'src/app/issue/util/git-host-terminology';
import {
    FailedStageError,
    FailedStageErrorConverter
} from '../../converter/failed-stage-error.converter';
import { ShortWorkflow } from '../../util/short-workflow';
import { SKILL_STAGES } from '../../constants/skill-stages.enum';

interface TimelineRow {
    stage: string;
    label: string; // i18n key for the stage name
    status: AgentStageStatus;
    statusKey: string; // i18n key for the status text
    glyph: string; // tabler icon name for the status
    noteKey: string | null; // i18n key for the outcome note
    attemptNo: number | null;
    agentName: string | null; // executor agent — only surfaced when a hand-off occurred
    at: string | null;
    approvedAt: string | null;
    model: string | null;
    isModelPlanned: boolean;
    modelFallbackFrom: string | null;
}

const STAGE_STATUS_KEY: Record<AgentStageStatus, string> = {
    pending: 'AGENT.TIMELINE.STATUS.PENDING',
    active: 'AGENT.TIMELINE.STATUS.ACTIVE',
    done: 'AGENT.TIMELINE.STATUS.DONE',
    awaiting_approval: 'AGENT.TIMELINE.STATUS.AWAITING_APPROVAL',
    failed: 'AGENT.TIMELINE.STATUS.FAILED',
    skipped: 'AGENT.TIMELINE.STATUS.SKIPPED'
};

const STAGE_STATUS_GLYPH: Record<AgentStageStatus, string> = {
    pending: 'circle',
    active: 'loader-2',
    done: 'circle-check',
    awaiting_approval: 'clock',
    failed: 'alert-circle',
    skipped: 'circle-minus'
};

const STAGE_NOTE_KEY: Record<string, string> = {
    no_clarifications: 'AGENT.TIMELINE.NOTE.NO_CLARIFICATIONS',
    submitted: 'AGENT.TIMELINE.NOTE.SUBMITTED',
    pr_opened: 'AGENT.TIMELINE.NOTE.PR_OPENED'
};

@Component({
    selector: 'app-agent-run-card',
    templateUrl: './agent-run-card.component.html',
    styleUrls: ['./agent-run-card.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
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

    protected readonly isStagePlanPopoverOpen = signal(false);
    protected readonly runStagePlan = signal<AgentRunStagePlanRes | null>(null);
    protected readonly skillCatalog = signal<Skill[]>([]);
    private stagePlanPopover: UiPopoverComponent | null = null;
    protected readonly stageStatus = signal<Record<string, UiSaveState>>({});

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

    // Drives the PR / MR section title so the card calls the linked change a
    // "Pull request" for github/gitea and "Merge request" for gitlab —
    // matches the terminology each host uses in its own UI. Falls back to
    // a generic label when prHostType is missing (older runs, edge cases).
    protected readonly prMrTermKey = computed(() =>
        GitHostTerminology.termKey((this.run()?.prHostType as HostType | null) ?? null)
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

    private readonly stageLabel = STAGE_LABELS as Record<string, string>;

    protected readonly stagePlanRows = computed<AgentStagePlanEditorRow[]>(() => {
        const stagePlan = this.runStagePlan();
        if (!stagePlan) {
            return [];
        }
        return stagePlan.stages.map(entry => ({
            stage: entry.name,
            idsSkill: entry.idsSkill,
            model: entry.model,
            maxTurns: entry.maxTurns,
            defaultMaxTurns:
                entry.name === AgentStage.Implementation
                    ? stagePlan.maxTurnsImplement
                    : stagePlan.maxTurnsPlan,
            lock: this.stagePlanLock(entry)
        }));
    });
    protected readonly runModel = computed(() => this.run()?.stagePlan?.model || null);

    protected readonly timeline = computed<TimelineRow[]>(() => {
        const stages = this.run()?.stages ?? [];
        const plannedModel = this.plannedModelByStage();
        // Agent provenance is only meaningful after a hand-off — i.e. when more
        // than one distinct agent executed stages. For single-agent runs the label
        // would be noise, so we suppress it.
        const distinctAgents = new Set(
            stages.map(s => s.idUserAgent).filter((id): id is number => id != null)
        );
        const showAgent = distinctAgents.size > 1;
        const users = this.usersMap();
        return stages.map((s: AgentStageProgress) => ({
            stage: s.stage,
            label: this.stageLabel[s.stage] ?? s.stage,
            status: s.status,
            statusKey: STAGE_STATUS_KEY[s.status] ?? s.status,
            glyph: STAGE_STATUS_GLYPH[s.status] ?? 'circle',
            noteKey: s.note ? (STAGE_NOTE_KEY[s.note] ?? null) : null,
            attemptNo: s.attemptNo && s.attemptNo > 1 ? s.attemptNo : null,
            agentName:
                showAgent && s.idUserAgent != null
                    ? (users.get(s.idUserAgent)?.name ?? null)
                    : null,
            at: s.at ?? null,
            approvedAt: s.approvedAt ?? null,
            model: s.model ?? (s.status === 'pending' ? (plannedModel.get(s.stage) ?? null) : null),
            isModelPlanned: !s.model && s.status === 'pending' && plannedModel.has(s.stage),
            modelFallbackFrom: s.modelFallbackFrom ?? null
        }));
    });

    private stagePlanLock(entry: AgentRunStagePlanEntry): AgentStagePlanEditorLock | null {
        if (entry.skip) {
            return AgentStagePlanEditorLock.Skipped;
        }
        return entry.dispatched ? AgentStagePlanEditorLock.Started : null;
    }

    private plannedModelByStage(): Map<string, string> {
        const plan = this.run()?.stagePlan;
        const planned = new Map<string, string>();
        (plan?.stages ?? [])
            .filter(entry => SKILL_STAGES.includes(entry.name as AgentStage))
            .forEach(entry => {
                const model = entry.model || plan?.model;
                if (model) {
                    planned.set(entry.name, model);
                }
            });
        return planned;
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

    protected onStagePlanPopoverClosed(): void {
        this.isStagePlanPopoverOpen.set(false);
    }

    protected onToggleStagePlanPopover(popover: UiPopoverComponent, event: Event): void {
        const willOpen = !this.isStagePlanPopoverOpen();
        this.isStagePlanPopoverOpen.set(willOpen);
        this.stagePlanPopover = willOpen ? popover : null;
        popover.toggle(event);
        if (willOpen) {
            this.onOpenStagePlan();
        }
    }

    protected onOpenStagePlan(): void {
        const idRun = this.run()?.idRun;
        if (idRun === undefined) {
            return;
        }
        this.skillApi
            .load$()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: catalog => {
                    this.skillCatalog.set(catalog);
                    this.repositionStagePlanPopover();
                },
                error: () => this.toast.showError('AGENT.SKILLS.LOAD_ERROR')
            });
        this.loadRunStagePlan(idRun);
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

    private loadRunStagePlan(idRun: number): void {
        this.agentRunApi
            .loadStagePlan$(idRun)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: payload => {
                    this.runStagePlan.set(payload);
                    this.repositionStagePlanPopover();
                },
                error: () => this.toast.showError('AGENT.RUN_STAGE_PLAN.LOAD_ERROR')
            });
    }

    private repositionStagePlanPopover(): void {
        this.stagePlanPopover?.reposition();
    }

    private setStageStatus(stage: string, status: UiSaveState): void {
        this.stageStatus.update(all => ({ ...all, [stage]: status }));
    }
}
