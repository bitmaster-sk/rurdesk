import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { SkillApi } from 'src/app/shared/api/skill.api.service';
import { Skill } from 'src/app/shared/model/skill.model';
import { AgentRunStageSkills } from '../../model/agent-run-skills.model';
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
import { prMrTermKey } from 'src/app/issue/util/pr-mr-term';
import { FailedStageError, resolveFailedStageError } from '../../util/failed-stage-error';

interface TimelineRow {
    label: string; // i18n key for the stage name
    status: AgentStageStatus;
    statusKey: string; // i18n key for the status text
    glyph: string; // tabler icon name for the status
    noteKey: string | null; // i18n key for the outcome note
    attemptNo: number | null;
    botName: string | null; // executor bot — only surfaced when a hand-off occurred
    at: string | null;
    approvedAt: string | null;
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

    public readonly cancelRun = output<void>();
    public readonly continueRun = output<void>();
    public readonly restartRun = output<void>();

    private readonly agentRunApi = inject(AgentRunApi);
    private readonly skillApi = inject(SkillApi);
    private readonly toast = inject(ToastNotificationService);
    private readonly destroyRef = inject(DestroyRef);

    protected readonly isSkillsPopoverOpen = signal(false);
    protected readonly runSkills = signal<AgentRunStageSkills[]>([]);
    protected readonly skillCatalog = signal<Skill[]>([]);
    private readonly stageStatus = signal<Record<string, UiSaveState>>({});

    protected readonly canEditSkills = computed(() => {
        const phase = this.run()?.phase;
        return (
            phase !== undefined &&
            phase !== AgentPhase.Done &&
            phase !== AgentPhase.Failed &&
            phase !== AgentPhase.Cancelled
        );
    });

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
        if (!run) return false;
        return (
            run.phase !== AgentPhase.Done &&
            run.phase !== AgentPhase.Failed &&
            run.phase !== AgentPhase.Cancelled &&
            run.phase !== AgentPhase.PrOpen
        );
    });

    protected readonly isWorking = computed(() => {
        const phase = this.run()?.phase;
        return phase === AgentPhase.InProgress;
    });

    // Drives the PR / MR section title so the card calls the linked change a
    // "Pull request" for github/gitea and "Merge request" for gitlab —
    // matches the terminology each host uses in its own UI. Falls back to
    // a generic label when prHostType is missing (older runs, edge cases).
    protected readonly prMrTermKey = computed(() =>
        prMrTermKey((this.run()?.prHostType as HostType | null) ?? null)
    );

    protected readonly isAwaitingInput = computed(
        () => this.run()?.phase === AgentPhase.AwaitingInput
    );

    // Surfaces the failed stage's error for the Failed-phase banner: a stable
    // i18n key derived from the reason code (AGENT.ERROR.<UPPER>) plus the raw
    // provider/agent detail. Null when the run hasn't failed with a reason.
    protected readonly failedStageError = computed<FailedStageError | null>(() =>
        resolveFailedStageError(this.run()?.stages ?? [])
    );

    private readonly stageLabel = STAGE_LABELS as Record<string, string>;

    protected readonly timeline = computed<TimelineRow[]>(() => {
        const stages = this.run()?.stages ?? [];
        // Bot provenance is only meaningful after a hand-off — i.e. when more
        // than one distinct bot executed stages. For single-bot runs the label
        // would be noise, so we suppress it.
        const distinctBots = new Set(
            stages.map(s => s.idUserBot).filter((id): id is number => id != null)
        );
        const showBot = distinctBots.size > 1;
        const users = this.usersMap();
        return stages.map((s: AgentStageProgress) => ({
            label: this.stageLabel[s.stage] ?? s.stage,
            status: s.status,
            statusKey: STAGE_STATUS_KEY[s.status] ?? s.status,
            glyph: STAGE_STATUS_GLYPH[s.status] ?? 'circle',
            noteKey: s.note ? (STAGE_NOTE_KEY[s.note] ?? null) : null,
            attemptNo: s.attemptNo && s.attemptNo > 1 ? s.attemptNo : null,
            botName: showBot && s.idUserBot != null ? (users.get(s.idUserBot)?.name ?? null) : null,
            at: s.at ?? null,
            approvedAt: s.approvedAt ?? null
        }));
    });

    protected get phaseSeverityForRun(): PhaseBadgeSeverity {
        const run = this.run();
        return run ? (this.phaseSeverity[run.phase] ?? 'secondary') : 'secondary';
    }

    protected get phaseLabelForRun(): string {
        const run = this.run();
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

    protected onSkillsPopoverClosed(): void {
        this.isSkillsPopoverOpen.set(false);
    }

    protected onToggleSkillsPopover(popover: UiPopoverComponent, event: Event): void {
        const willOpen = !this.isSkillsPopoverOpen();
        this.isSkillsPopoverOpen.set(willOpen);
        popover.toggle(event);
        if (willOpen) {
            this.onOpenSkills();
        }
    }

    protected onOpenSkills(): void {
        const idRun = this.run()?.idRun;
        if (idRun === undefined) {
            return;
        }
        this.skillApi
            .load$()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: catalog => this.skillCatalog.set(catalog),
                error: () => this.toast.showError('AGENT.SKILLS.LOAD_ERROR')
            });
        this.loadRunSkills(idRun);
    }

    protected skillStageLabel(stage: AgentStage): string {
        return STAGE_LABELS[stage] ?? stage;
    }

    protected stageSaveStatus(stage: string): UiSaveState {
        return this.stageStatus()[stage] ?? UiSaveState.Idle;
    }

    protected isSkillOn(stage: AgentRunStageSkills, idSkill: number): boolean {
        return stage.idsSkill.includes(idSkill);
    }

    protected onToggleSkill(stage: AgentRunStageSkills, idSkill: number): void {
        const idRun = this.run()?.idRun;
        if (idRun === undefined || stage.dispatched) {
            return;
        }
        const next = stage.idsSkill.includes(idSkill)
            ? stage.idsSkill.filter(idPlanned => idPlanned !== idSkill)
            : [...stage.idsSkill, idSkill];

        this.setStageStatus(stage.name, UiSaveState.Saving);
        this.agentRunApi
            .patchAgentRunSkills$(idRun, stage.name, next)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: payload => {
                    this.runSkills.set(payload);
                    this.setStageStatus(stage.name, UiSaveState.Saved);
                },
                error: (err: { status?: number }) => {
                    this.setStageStatus(stage.name, UiSaveState.Error);
                    if (err?.status === 409) {
                        // The scheduler dispatched the stage between render and click.
                        this.loadRunSkills(idRun);
                        this.toast.showError('AGENT.RUN_SKILLS.RACE_ERROR');
                        return;
                    }
                    this.toast.showError('AGENT.RUN_SKILLS.SAVE_ERROR');
                }
            });
    }

    private loadRunSkills(idRun: number): void {
        this.agentRunApi
            .getAgentRunSkills$(idRun)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: payload => this.runSkills.set(payload),
                error: () => this.toast.showError('AGENT.RUN_SKILLS.LOAD_ERROR')
            });
    }

    private setStageStatus(stage: string, status: UiSaveState): void {
        this.stageStatus.update(all => ({ ...all, [stage]: status }));
    }
}
