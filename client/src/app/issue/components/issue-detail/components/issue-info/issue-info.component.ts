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
import { filter, switchMap, takeUntil } from 'rxjs/operators';
import { toSignal } from '@angular/core/rxjs-interop';
import { ElementRef } from '@angular/core';
import { MessageEditorComponent } from 'src/app/message/components/message-editor/message-editor.component';
import { Project } from 'src/app/project/model/project.model';
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
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { MrDiffApi } from 'src/app/issue/api/mr-diff.api.service';
import { GitIntegrationApi } from 'src/app/project/api/git-integration.api.service';
import {
    GitIntegrationRes,
    MrDiff,
    MrDiffFile,
    MrStatus
} from 'src/app/project/model/git-integration.model';
import { prMrLinkTitleKey, prMrTermKey } from 'src/app/issue/util/pr-mr-term';
import { buildGitHostMrFilesUrl } from 'src/app/issue/util/git-host-file-url';
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
export class IssueInfoComponent {
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
    private readonly i18n = inject(I18nService);
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

    public readonly currentIssue = signal<Issue | null>(null);
    public readonly isNewIssue = computed(() => !this.currentIssue()?.idIssue);

    /** Whole-issue auto-save status (one form → one save), shown in the header. */
    public readonly saveStatus = signal<UiSaveState>(UiSaveState.Idle);

    public currentUserId(): number {
        return this.authStore.getUser().idUser;
    }
    public readonly showInputTitle = signal(false);
    public readonly showInputDescription = signal(false);

    protected readonly isMrLinkPickerOpen = signal(false);
    // PR collapsible panel — default closed so the issue panel isn't
    // dominated by the diff. The diff fetch is deferred until the first
    // expand, matching the previous "load on toggle" behavior.
    protected readonly isPrPanelCollapsed = signal(true);
    protected readonly mrStatus = signal<MrStatus | null>(null);
    protected readonly mrDiff = signal<MrDiff | null>(null);
    protected readonly isMrDiffLoading = signal(false);
    // Loaded on demand the first time the issue's idGitIntegration is seen
    // so the panel can render host-appropriate labels ("Pull request" vs
    // "Merge request") and the link picker / unlink action match.
    protected readonly gitIntegration = signal<GitIntegrationRes | null>(null);
    protected readonly mrTermKey = computed(() =>
        prMrTermKey(this.gitIntegration()?.hostType ?? null)
    );
    protected readonly mrLinkTitleKey = computed(() =>
        prMrLinkTitleKey(this.gitIntegration()?.hostType ?? null)
    );
    protected readonly mrLink = computed(
        () => this.agentRun()?.prUrl ?? (this.mrStatus()?.webUrl || null)
    );

    /**
     * Per-file deep-link generator handed to the diff viewer so each file
     * header can render an "open on host" icon. All files in the same MR
     * point at the same files-changed URL on the host — we don't compute
     * per-file anchors (would need async hashing of the path). Returns null
     * until the issue actually has an MR id + the git integration is
     * loaded; the viewer then just skips the icon.
     */
    protected readonly mrFileLinkBuilder = computed<DiffFileLinkBuilder | null>(() => {
        const integration = this.gitIntegration();
        const mrId = this.currentIssue()?.mrId;
        if (!integration || !mrId) return null;
        const url = buildGitHostMrFilesUrl(
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
            label: this.i18n.instant('AI.SINGULAR'),
            items: [
                {
                    label: this.i18n.instant('SPLIT.SINGULAR'),
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
            label: this.i18n.instant('ISSUE.PIN.SINGULAR'),
            items: [
                {
                    label: this.i18n.instant('ISSUE.PIN.TO.PROJECT.PAGE'),
                    command: () => this.onPin(PinDestinationType.PROJECT)
                },
                {
                    label: this.i18n.instant('ISSUE.PIN.TO.MY.PAGE'),
                    command: () => this.onPin(PinDestinationType.USER)
                }
            ]
        }
    ];

    public form: FormGroup = new FormGroup({});

    private readonly formReset$ = new Subject<void>();

    public constructor() {
        this.destroyRef.onDestroy(() => this.formReset$.complete());

        effect(() => {
            const issue = this.issue();
            this.currentIssue.set(issue);
            untracked(() => {
                this.form = this.issueToForm();
                this.listenFormChange();
                this.mrStatus.set(null);
                this.mrDiff.set(null);
                this.isPrPanelCollapsed.set(true);
                this.gitIntegration.set(null);
                if (issue?.idGitIntegration && issue.mrId) {
                    this.loadMrStatus(issue.idGitIntegration, issue.mrId);
                }
                if (issue?.idGitIntegration) {
                    this.loadGitIntegration(issue.idProject, issue.idGitIntegration);
                }
            });
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
        // Only an edit auto-saves and shows the header chip; a new issue is an
        // explicit create that navigates away on success.
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
            this.mrStatus.set(null);
            this.mrDiff.set(null);
            this.isPrPanelCollapsed.set(true);
            if (saved.idGitIntegration && saved.mrId) {
                this.loadMrStatus(saved.idGitIntegration, saved.mrId);
            }
        });
    }

    protected onMrLinkCancelled(): void {
        this.isMrLinkPickerOpen.set(false);
    }

    protected onTogglePrPanel(): void {
        const wasCollapsed = this.isPrPanelCollapsed();
        this.isPrPanelCollapsed.set(!wasCollapsed);
        if (!wasCollapsed) return;

        // Lazy-load the diff the first time the panel is opened so we don't
        // hit the git host on every issue render.
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

    public get assignedToControl(): FormControl {
        return this.form.get('assignedTo') as FormControl;
    }

    public get idSeverityControl(): FormControl {
        return this.form.get('idSeverity') as FormControl;
    }

    public get idIssueTypeControl(): FormControl {
        return this.form.get('idIssueType') as FormControl;
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

    private issueToForm(): FormGroup {
        const issue = this.currentIssue();
        const project = this.project();
        return this.fb.group({
            idIssue: this.fb.control(issue?.idIssue),
            idIssuePublic: this.fb.control(issue?.idIssuePublic),
            idProject: this.fb.control(issue?.idProject),
            idState: this.fb.control(this.isNewIssue() ? project?.idStateDefault : issue?.idState),
            idSeverity: this.fb.control(
                this.isNewIssue() ? project?.idSeverityDefault : issue?.idSeverity
            ),
            idIssueType: this.fb.control(
                this.isNewIssue() ? project?.idIssueTypeDefault : issue?.idIssueType
            ),
            title: this.fb.control(issue?.title, {
                validators: [Validators.required, Validators.maxLength(100)],
                updateOn: 'blur'
            }),
            description: this.fb.control(issue?.description, {
                validators: [Validators.required],
                updateOn: 'blur'
            }),
            assignedTo: this.fb.control(issue?.assignedTo),
            estimated: this.fb.control(
                DurationFormatter.durationToString(
                    DurationConverter.secondsToDuration(issue?.estimated ?? 0)
                ),
                { validators: [DurationValidator.duration], updateOn: 'blur' }
            ),
            points: this.fb.control(issue?.points ?? null, {
                validators: [Validators.min(0)],
                updateOn: 'blur'
            }),
            scheduledAt: [issue?.scheduledAt]
        });
    }

    private formToIssue(): Issue {
        const issue: Issue = { ...(this.form.value as Issue) };
        issue.estimated = DurationConverter.durationToSeconds(
            DurationParser.stringToDuration(this.form.value.estimated)
        );
        const points = this.form.value.points;
        issue.points =
            points === null || points === undefined || points === '' ? null : Number(points);
        return issue;
    }
}
