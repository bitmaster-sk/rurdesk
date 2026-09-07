import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    effect,
    inject,
    input,
    output,
    signal,
    untracked,
    viewChild
} from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { Subject } from 'rxjs';

interface IssueInfoForm {
    idIssue: FormControl<number | null>;
    idIssuePublic: FormControl<number | null>;
    idProject: FormControl<number | null>;
    idState: FormControl<number | null>;
    idSeverity: FormControl<number | null>;
    idIssueType: FormControl<number | null>;
    title: FormControl<string | null>;
    description: FormControl<string | null>;
    assignedTo: FormControl<number | null>;
    estimated: FormControl<string | null>;
    points: FormControl<number | null>;
    scheduledAt: FormControl<Date | null>;
}

import { filter, switchMap, takeUntil } from 'rxjs/operators';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { ElementRef, OnInit } from '@angular/core';
import { MessageEditorComponent } from 'src/app/message/components/message-editor/message-editor.component';
import { Project } from 'src/app/project/model/project.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { IssueService } from '../../../../issue.service';
import { Issue } from '../../../../model/issue.model';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { DurationParser } from 'src/app/shared/duration/duration.parser';
import { DurationValidator } from 'src/app/shared/duration/duration.validator';
import { StateStore } from 'src/app/state/store/state.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';
import { PinService } from 'src/app/pin/pin.service';
import { PinDestinationType } from 'src/app/pin/constant/pin-destination-type.enum';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { ProjectStore } from 'src/app/project/project.store';
import { MrDiffApi } from 'src/app/issue/api/mr-diff.api.service';
import { GitIntegrationApi } from 'src/app/project/api/git-integration.api.service';
import {
    GitIntegrationRes,
    MrDiff,
    MrDiffFile,
    MrStatus
} from 'src/app/project/model/git-integration.model';
import { GitHostTerminology } from 'src/app/issue/util/git-host-terminology';
import { GitHostUrl } from 'src/app/issue/util/git-host-file-url';
import { DiffFileLinkBuilder } from 'src/app/shared/components/diff-viewer/diff-viewer.component';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';

@Component({
    selector: 'app-issue-info',
    templateUrl: './issue-info.component.html',
    styleUrls: ['./issue-info.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class IssueInfoComponent implements OnInit {
    private readonly inputTitle = viewChild<ElementRef<HTMLInputElement>>('inputTitle');
    private readonly inputDescription = viewChild<MessageEditorComponent>('inputDescription');

    public readonly issue = input<Issue | null>(null);
    public readonly project = input<Project | null>(null);
    public readonly agentRun = input<AgentRun | null>(null);
    public readonly splitRequested = output<Issue>();
    public readonly trackAdded = output<Track>();
    public readonly cancelAgentRun = output<void>();
    public readonly continueAgentRun = output<void>();
    public readonly restartAgentRun = output<void>();

    private readonly router = inject(Router);
    private readonly fb = inject(FormBuilder);
    private readonly sIssue = inject(IssueService);
    private readonly sPin = inject(PinService);
    private readonly authStore = inject(AuthStore);
    private readonly stateStore = inject(StateStore);
    private readonly severityStore = inject(SeverityStore);
    private readonly issueTypeStore = inject(IssueTypeStore);
    private readonly projectMemberStore = inject(ProjectMemberStore);
    private readonly projectStore = inject(ProjectStore);
    private readonly mrDiffApi = inject(MrDiffApi);
    private readonly gitIntegrationApi = inject(GitIntegrationApi);
    private readonly destroyRef = inject(DestroyRef);
    private readonly notice = inject(NoticeService);

    public readonly currentIssue = signal<Issue | null>(null);
    public readonly isNewIssue = computed(() => !this.currentIssue()?.idIssue);

    // Auto-save status of the whole issue form, shown in the header.
    public readonly saveStatus = signal<UiSaveState>(UiSaveState.Idle);

    public currentUserId(): number {
        return this.authStore.getUser().idUser;
    }
    public readonly showInputTitle = signal(false);
    public readonly showInputDescription = signal(false);

    protected readonly isMrLinkPickerOpen = signal(false);
    // PR panel starts closed; the diff is fetched on the first expand.
    protected readonly isPrPanelCollapsed = signal(true);
    protected readonly mrStatus = signal<MrStatus | null>(null);
    protected readonly mrDiff = signal<MrDiff | null>(null);
    protected readonly isMrDiffLoading = signal(false);
    // Git integration of the linked MR; gives the panel the host name and labels.
    protected readonly gitIntegration = signal<GitIntegrationRes | null>(null);
    protected readonly mrTermKey = computed(() =>
        GitHostTerminology.termKey(this.gitIntegration()?.hostType ?? null)
    );
    protected readonly mrLinkTitleKey = computed(() =>
        GitHostTerminology.linkTitleKey(this.gitIntegration()?.hostType ?? null)
    );
    protected readonly mrLink = computed(
        () => this.agentRun()?.prUrl ?? (this.mrStatus()?.webUrl || null)
    );

    // Builds the "open on host" link for each file in the diff. Every file gets
    // the same files-changed URL; null until the MR and integration are known.
    protected readonly mrFileLinkBuilder = computed<DiffFileLinkBuilder | null>(() => {
        const integration = this.gitIntegration();
        const mrId = this.currentIssue()?.mrId;
        if (!integration || !mrId) return null;
        const url = GitHostUrl.buildMrFilesUrl(
            integration.hostType,
            integration.baseUrl,
            integration.repoPath,
            mrId
        );
        return (_file: MrDiffFile) => url;
    });

    public readonly states = toSignal(
        this.projectStore.project$.pipe(
            switchMap(project => this.stateStore.statesByProject$(project.idProject))
        ),
        { initialValue: [] }
    );

    public readonly severities = toSignal(
        this.projectStore.project$.pipe(
            switchMap(project => this.severityStore.severitiesByProject$(project.idProject))
        ),
        { initialValue: [] }
    );

    public readonly issueTypes = toSignal(
        this.projectStore.project$.pipe(
            switchMap(project => this.issueTypeStore.issueTypesByProject$(project.idProject))
        ),
        { initialValue: [] }
    );

    public readonly users = toSignal(this.projectMemberStore.users$, { initialValue: [] });

    public readonly usersMap = toSignal(this.projectMemberStore.usersMap$, {
        initialValue: new Map()
    });

    public readonly actions: UiMenuItem[] = [
        {
            labelKey: 'AI.SINGULAR',
            items: [
                {
                    labelKey: 'SPLIT.SINGULAR',
                    command: () => {
                        const issue = this.currentIssue();
                        if (issue) {
                            this.splitRequested.emit(issue);
                        }
                    }
                }
            ]
        },
        {
            labelKey: 'ISSUE.PIN.SINGULAR',
            items: [
                {
                    labelKey: 'ISSUE.PIN.TO.PROJECT.PAGE',
                    command: () => this.onPin(PinDestinationType.PROJECT)
                },
                {
                    labelKey: 'ISSUE.PIN.TO.MY.PAGE',
                    command: () => this.onPin(PinDestinationType.USER)
                }
            ]
        }
    ];

    public form: FormGroup<IssueInfoForm> = this.issueToForm();

    private readonly formReset$ = new Subject<void>();

    // What the panel currently shows and what was already requested. Every
    // issue notice swaps in a new issue object, so without these keys the panel
    // would reset and refetch on unrelated edits.
    private gitMrKey: string | null = null;
    private gitIntegrationKey: number | null = null;

    public constructor() {
        this.destroyRef.onDestroy(() => this.formReset$.complete());

        effect(() => {
            const issue = this.issue();
            this.currentIssue.set(issue);
            untracked(() => {
                this.form = this.issueToForm();
                this.listenFormChange();
                this.syncMrPanel(issue);
            });
        });
    }

    public ngOnInit(): void {
        this.listenMrStatusChange();
    }

    // Resets and reloads the PR panel, but only when the issue points at a
    // different MR or integration than what is already shown.
    private syncMrPanel(issue: Issue | null): void {
        const gitMrKey =
            issue?.idGitIntegration && issue.mrId
                ? `${issue.idIssue}:${issue.idGitIntegration}:${issue.mrId}`
                : null;

        if (gitMrKey !== this.gitMrKey) {
            this.gitMrKey = gitMrKey;
            this.mrStatus.set(null);
            this.mrDiff.set(null);
            this.isPrPanelCollapsed.set(true);
            if (issue?.idGitIntegration && issue.mrId) {
                this.loadMrStatus(issue.idGitIntegration, issue.mrId);
            }
        }

        const idGitIntegration = issue?.idGitIntegration ?? null;
        if (idGitIntegration !== this.gitIntegrationKey) {
            this.gitIntegrationKey = idGitIntegration;
            this.gitIntegration.set(null);
            if (issue && idGitIntegration) {
                this.loadGitIntegration(issue.idProject, idGitIntegration);
            }
        }
    }

    private listenMrStatusChange(): void {
        this.notice.mrStatus$
            .pipe(
                filter(n => n.payload.idIssue === this.currentIssue()?.idIssue),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(n => {
                const issue = this.currentIssue();
                if (!issue?.idGitIntegration || !issue.mrId) {
                    return;
                }
                if (
                    n.payload.idGitIntegration !== issue.idGitIntegration ||
                    n.payload.idMr !== issue.mrId
                ) {
                    return;
                }
                const status: MrStatus = {
                    state: n.payload.state as MrStatus['state'],
                    approved: n.payload.approved,
                    ciStatus: n.payload.ciStatus as MrStatus['ciStatus'],
                    webUrl: n.payload.webUrl,
                    headSha: n.payload.headSha
                };
                this.mrStatus.set(status);
            });
    }

    private loadGitIntegration(idProject: number, idGitIntegration: number): void {
        this.gitIntegrationApi.get$(idProject, idGitIntegration).subscribe({
            next: integration => this.gitIntegration.set(integration),
            error: () => this.gitIntegration.set(null)
        });
    }

    public onSave(): void {
        const issue = this.formToIssue();
        const isUpdate = !!issue.idIssue;
        // Only an edit shows the save chip; a create navigates away instead.
        if (isUpdate) {
            this.saveStatus.set(UiSaveState.Saving);
        }
        const saver = isUpdate ? this.sIssue.updateIssue(issue) : this.sIssue.insertIssue(issue);
        saver.subscribe({
            next: savedIssue => {
                if (isUpdate) {
                    this.currentIssue.set(savedIssue);
                    this.refreshFormValues(savedIssue);
                    this.saveStatus.set(UiSaveState.Saved);
                } else {
                    void this.router.navigate([
                        '/project',
                        savedIssue.idProject,
                        'issue',
                        savedIssue.idIssuePublic
                    ]);
                }
            },
            error: () => {
                if (isUpdate) {
                    this.saveStatus.set(UiSaveState.Error);
                }
            }
        });
    }

    public onToggleInputTitle(): void {
        this.showInputTitle.update(v => !v);
        if (this.showInputTitle()) {
            setTimeout(() => this.inputTitle()?.nativeElement.focus(), 0);
        }
    }

    public onInputTitleFocusout(): void {
        this.showInputTitle.set(false);
    }

    public onToggleInputDescription(): void {
        this.showInputDescription.update(v => !v);
        if (this.showInputDescription()) {
            setTimeout(() => this.inputDescription()?.focus(), 0);
        }
    }

    public onInputDescriptionChange(): void {
        this.showInputDescription.set(false);
    }

    public onTrackAdded(track: Track): void {
        this.currentIssue.update(issue =>
            issue ? { ...issue, tracked: (issue.tracked ?? 0) + (track.tracked ?? 0) } : issue
        );
        this.trackAdded.emit(track);
    }

    public onPin(idPinDestinationType: PinDestinationType): void {
        const issue = this.currentIssue();
        if (!issue) {
            return;
        }
        const idPinDestination =
            idPinDestinationType === PinDestinationType.PROJECT
                ? issue.idProject
                : this.authStore.getUser().idUser;
        this.sPin
            .insertPin({
                idPinDestination,
                idPinDestinationType,
                idIssue: issue.idIssue
            })
            .subscribe();
    }

    protected onOpenMrLinkPicker(): void {
        this.isMrLinkPickerOpen.set(true);
    }

    protected onMrLinked(result: { idGitIntegration: number; mrId: string } | null): void {
        this.isMrLinkPickerOpen.set(false);
        const issue = this.currentIssue();
        if (!issue) return;
        const updated: Issue = {
            ...issue,
            idGitIntegration: result?.idGitIntegration ?? null,
            mrId: result?.mrId ?? null
        };
        this.sIssue.updateIssue(updated).subscribe(saved => {
            this.currentIssue.set(saved);
            this.syncMrPanel(saved);
        });
    }

    // Syncs the assignee the dock already saved. Must not emit: the form
    // autosaves, so a stale assignedTo would be sent back and un-assign the agent.
    protected onAgentRunCreated(run: AgentRun): void {
        this.assignedToControl.setValue(run.idUserAgent, { emitEvent: false });
        const issue = this.currentIssue();
        if (issue) {
            this.currentIssue.set({ ...issue, assignedTo: run.idUserAgent });
        }
    }

    protected onMrLinkCancelled(): void {
        this.isMrLinkPickerOpen.set(false);
    }

    protected onTogglePrPanel(): void {
        const wasCollapsed = this.isPrPanelCollapsed();
        this.isPrPanelCollapsed.set(!wasCollapsed);
        if (!wasCollapsed) return;

        // Load the diff on first expand, not on every issue render.
        const issue = this.currentIssue();
        if (!issue?.idGitIntegration || !issue.mrId || this.mrDiff()) return;
        this.isMrDiffLoading.set(true);
        this.mrDiffApi.getDiff$(issue.idProject, issue.idGitIntegration, issue.mrId).subscribe({
            next: diff => {
                this.mrDiff.set(diff);
                this.isMrDiffLoading.set(false);
            },
            error: () => this.isMrDiffLoading.set(false)
        });
    }

    private loadMrStatus(idGitIntegration: number, mrId: string): void {
        const issue = this.currentIssue();
        if (!issue) return;
        this.mrDiffApi
            .getStatus$(issue.idProject, idGitIntegration, mrId)
            .subscribe(status => this.mrStatus.set(status));
    }

    public get assignedToControl(): FormControl<number | null> {
        return this.form.controls.assignedTo;
    }

    public get idSeverityControl(): FormControl<number | null> {
        return this.form.controls.idSeverity;
    }

    public get idIssueTypeControl(): FormControl<number | null> {
        return this.form.controls.idIssueType;
    }

    private listenFormChange(): void {
        this.formReset$.next();
        this.form.valueChanges
            .pipe(
                filter(() => !this.isNewIssue() && this.form.valid),
                takeUntil(this.formReset$)
            )
            .subscribe(() => this.onSave());
    }

    private refreshFormValues(issue: Issue): void {
        this.form.patchValue(
            {
                estimated: DurationFormatter.durationToString(
                    DurationConverter.secondsToDuration(issue.estimated ?? 0)
                ),
                points: issue.points ?? null
            },
            { emitEvent: false }
        );
    }

    private issueToForm(): FormGroup<IssueInfoForm> {
        const issue = this.currentIssue();
        const project = this.project();
        return this.fb.group<IssueInfoForm>({
            idIssue: this.fb.control<number | null>(issue?.idIssue ?? null),
            idIssuePublic: this.fb.control<number | null>(issue?.idIssuePublic ?? null),
            idProject: this.fb.control<number | null>(issue?.idProject ?? null),
            idState: this.fb.control<number | null>(
                this.isNewIssue() ? (project?.idStateDefault ?? null) : (issue?.idState ?? null)
            ),
            idSeverity: this.fb.control<number | null>(
                this.isNewIssue()
                    ? (project?.idSeverityDefault ?? null)
                    : (issue?.idSeverity ?? null)
            ),
            idIssueType: this.fb.control<number | null>(
                this.isNewIssue()
                    ? (project?.idIssueTypeDefault ?? null)
                    : (issue?.idIssueType ?? null)
            ),
            title: this.fb.control<string | null>(issue?.title ?? null, {
                validators: [Validators.required, Validators.maxLength(100)],
                updateOn: 'blur'
            }),
            description: this.fb.control<string | null>(issue?.description ?? null, {
                validators: [Validators.required],
                updateOn: 'blur'
            }),
            assignedTo: this.fb.control<number | null>(issue?.assignedTo ?? null),
            estimated: this.fb.control<string | null>(
                DurationFormatter.durationToString(
                    DurationConverter.secondsToDuration(issue?.estimated ?? 0)
                ),
                { validators: [DurationValidator.duration], updateOn: 'blur' }
            ),
            points: this.fb.control<number | null>(issue?.points ?? null, {
                validators: [Validators.min(0)],
                updateOn: 'blur'
            }),
            scheduledAt: this.fb.control<Date | null>(issue?.scheduledAt ?? null)
        });
    }

    private formToIssue(): Issue {
        const v = this.form.getRawValue();
        const current = this.currentIssue();
        return {
            idIssue: v.idIssue ?? 0,
            idIssuePublic: v.idIssuePublic ?? 0,
            idProject: v.idProject ?? 0,
            idState: v.idState,
            idSeverity: v.idSeverity,
            idIssueType: v.idIssueType,
            title: v.title ?? '',
            description: v.description ?? '',
            assignedTo: v.assignedTo,
            estimated: DurationConverter.durationToSeconds(
                DurationParser.stringToDuration(v.estimated ?? '')
            ),
            points: v.points ?? null,
            scheduledAt: v.scheduledAt,
            tracked: current?.tracked ?? 0
        };
    }
}
