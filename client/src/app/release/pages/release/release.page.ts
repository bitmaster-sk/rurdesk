import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    effect,
    inject,
    signal,
    untracked,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { NonNullableFormBuilder, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import isEqual from 'lodash-es/isEqual';
import { EMPTY, Observable, merge, of } from 'rxjs';
import { catchError, debounceTime, filter, finalize, map, switchMap, tap } from 'rxjs/operators';
import { AgentRunApi } from 'src/app/agent/api/agent-run.api.service';
import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { AgentThinkingStore } from 'src/app/agent/store/agent-thinking.store';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AclStore } from 'src/app/project/store/acl.store';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TagStore } from 'src/app/tag/store/tag.store';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { UiStepperStep } from 'src/app/ui/components/stepper/stepper.component';
import { WikiEditorComponent } from 'src/app/wiki/components/wiki-editor/wiki-editor.component';
import { ReleaseApi } from '../../api/release.api.service';
import { RELEASE_NAME_MAX_LENGTH } from '../../constants/release.constants';
import { ReleaseNotesMode } from '../../constants/release-notes-mode.enum';
import { ReleaseOpenIssues } from '../../constants/release-open-issues.enum';
import { ReleaseStatus } from '../../constants/release-status.enum';
import { ReleaseStep } from '../../constants/release-step.enum';
import { ReleaseNotesConverter } from '../../converter/release-notes.converter';
import { ReleaseForm } from '../../entity/release-form.entity';
import { ReleaseTaskRow } from '../../entity/release-task-row.entity';
import {
    ReleaseDraft,
    ReleaseView,
    StartReleaseAgentReq,
    UpdateReleaseReq
} from '../../model/release.model';
import { ReleaseTaskService } from '../../service/release-task.service';

@Component({
    selector: 'app-release',
    templateUrl: './release.page.html',
    styleUrls: ['./release.page.scss'],
    providers: [AgentThinkingStore],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleasePage {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly api = inject(ReleaseApi);
    private readonly taskService = inject(ReleaseTaskService);
    private readonly agentRunApi = inject(AgentRunApi);
    private readonly thinkingStore = inject(AgentThinkingStore);
    private readonly i18n = inject(I18nService);
    private readonly destroyRef = inject(DestroyRef);
    private readonly fb = inject(NonNullableFormBuilder);
    protected readonly acl = inject(AclStore);

    protected readonly Step = ReleaseStep;
    protected readonly NotesMode = ReleaseNotesMode;
    protected readonly SaveState = UiSaveState;
    protected readonly idProject = Number(this.route.snapshot.paramMap.get('idProject'));
    private readonly idRelease = Number(this.route.snapshot.paramMap.get('idRelease'));

    protected readonly form = this.fb.group<ReleaseForm>({
        name: this.fb.control('', [
            Validators.required,
            Validators.maxLength(RELEASE_NAME_MAX_LENGTH),
            Validators.pattern(/\S/)
        ]),
        recreateTag: this.fb.control(true),
        openIssues: this.fb.control(ReleaseOpenIssues.Move),
        notes: this.fb.control('')
    });
    private readonly formValue = signal(this.form.getRawValue());

    protected readonly release = signal<ReleaseView | null>(null);
    protected readonly draft = signal<ReleaseDraft | null>(null);
    protected readonly step = signal<ReleaseStep>(
        Number(this.route.snapshot.queryParamMap.get('step') ?? ReleaseStep.Version)
    );
    protected readonly saveState = signal(UiSaveState.Idle);
    protected readonly isBusy = signal(false);
    protected readonly isEditingNotes = signal(false);
    protected readonly tasks = signal<ReleaseTaskRow[] | null>(null);
    protected readonly notesMode = signal(ReleaseNotesMode.Choose);
    protected readonly notesRun = signal<AgentRun | null>(null);
    private readonly idRunErrorDismissed = signal<number | null>(null);
    protected readonly isDeleteOpen = signal(false);
    private lastSaved: Required<UpdateReleaseReq> | null = null;

    private readonly editor = viewChild(WikiEditorComponent);
    private readonly users = toSignal(inject(ProjectMemberStore).users$, { initialValue: [] });
    private readonly tags = toSignal(inject(TagStore).tagsByProject$(this.idProject), {
        initialValue: []
    });

    protected readonly agents = computed<User[]>(() => this.users().filter(user => user.isAgent));
    protected readonly tag = computed(
        () => this.tags().find(tag => tag.idTag === this.release()?.idTag) ?? null
    );
    protected readonly isDraft = computed(() => this.release()?.status === ReleaseStatus.Draft);
    protected readonly canEdit = computed(() => this.isDraft() && this.acl.canUpdateProject());
    protected readonly isNotesPending = computed(() => !!this.release()?.isNotesPending);
    protected readonly isTagDeleted = computed(
        () => this.isDraft() && this.release()?.idTag === null
    );
    protected readonly notesStage = computed(
        () =>
            this.notesRun()?.stages?.find(
                stage => (stage.stage as AgentStage) === AgentStage.ReleaseNotes
            ) ?? null
    );
    protected readonly writingAgent = computed(
        () => this.users().find(user => user.idUser === this.notesRun()?.idUserAgent) ?? null
    );
    protected readonly notesRunError = computed<string | null>(() => {
        const release = this.release();
        if (
            release?.notesRunPhase !== AgentPhase.Failed ||
            release.idRunNotes === this.idRunErrorDismissed()
        ) {
            return null;
        }
        return release.notesRunError ?? '';
    });
    protected readonly name = computed(() => (this.formValue().name ?? '').trim());
    protected readonly notes = computed(() => this.formValue().notes ?? '');
    protected readonly isRecreateTag = computed(() => this.formValue().recreateTag);
    protected readonly closedCount = computed(
        () => this.draft()?.sections.reduce((sum, section) => sum + section.issues.length, 0) ?? 0
    );
    protected readonly steps = computed<UiStepperStep[]>(() => [
        { label: this.i18n.instant('RELEASE.STEP.VERSION') },
        { label: this.i18n.instant('RELEASE.STEP.TASKS') },
        { label: this.i18n.instant('RELEASE.STEP.NOTES') }
    ]);
    protected readonly isNameSameAsTag = computed(() => {
        const tagName = this.release()?.tagName;
        return (
            this.isRecreateTag() && !!tagName && this.name().toLowerCase() === tagName.toLowerCase()
        );
    });
    protected readonly publishBlocker = computed<string | null>(() => {
        this.formValue();
        if (this.isTagDeleted()) {
            return 'RELEASE.BLOCKER.TAG';
        }
        if (this.isNotesPending()) {
            return 'RELEASE.BLOCKER.AGENT';
        }
        if (this.form.invalid) {
            return 'RELEASE.BLOCKER.NAME';
        }
        if (this.isNameSameAsTag()) {
            return 'RELEASE.BLOCKER.SAME_NAME';
        }
        if (!this.notes().trim()) {
            return 'RELEASE.BLOCKER.NOTES';
        }
        return null;
    });
    protected readonly authorName = computed(() => {
        const idUser = this.release()?.releasedBy;
        return this.users().find(user => user.idUser === idUser)?.name ?? '—';
    });

    public constructor() {
        this.load();
        this.form.valueChanges
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.formValue.set(this.form.getRawValue()));
        this.form.valueChanges
            .pipe(
                debounceTime(600),
                filter(() => this.isDirty()),
                switchMap(() => this.save$()),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe();
        const notices = inject(NoticeService);
        notices.release$
            .pipe(
                filter(notice => notice.payload.idRelease === this.idRelease),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(notice => {
                if (notice.action === NoticeAction.Delete) {
                    void this.router.navigate(['/project', this.idProject, 'release']);
                }
            });
        merge(
            notices.release$.pipe(
                filter(
                    notice =>
                        notice.payload.idRelease === this.idRelease &&
                        notice.action !== NoticeAction.Delete
                )
            ),
            notices.agentRun$.pipe(
                filter(() => this.isNotesPending()),
                tap(notice => this.applyRun(notice.payload as AgentRun | null))
            )
        )
            .pipe(
                debounceTime(300),
                switchMap(() =>
                    this.api.loadOne$(this.idProject, this.idRelease).pipe(catchError(() => EMPTY))
                ),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe(release => this.applyRelease(release));
        effect(() => {
            if (this.canEdit()) {
                this.form.enable({ emitEvent: false });
            } else {
                this.form.disable({ emitEvent: false });
            }
            this.formValue.set(this.form.getRawValue());
        });
        effect(() => {
            const step = this.step();
            untracked(() => this.onStepEntered(step));
        });
        effect(() => {
            const idRun = this.isNotesPending() ? (this.release()?.idRunNotes ?? null) : null;
            untracked(() => this.bindRun(idRun));
        });
    }

    protected onNext(): void {
        this.step.set(this.step() + 1);
    }

    protected onEditorChange(notes: string): void {
        this.form.controls.notes.setValue(notes);
    }

    protected onWriteMyself(): void {
        const draft = this.draft();
        if (!this.notes().trim() && draft) {
            this.form.controls.notes.setValue(
                ReleaseNotesConverter.toMarkdown(draft.sections, this.i18n.instant('RELEASE.OTHER'))
            );
        }
        this.notesMode.set(ReleaseNotesMode.Editor);
    }

    protected onWriteWithAgent(): void {
        this.notesMode.set(ReleaseNotesMode.Agent);
    }

    protected onAgentCancelled(): void {
        this.notesMode.set(this.notes().trim() ? ReleaseNotesMode.Editor : ReleaseNotesMode.Choose);
    }

    protected onAgentStarted(req: StartReleaseAgentReq): void {
        this.runAfterSave(() => this.api.startAgent$(this.idProject, this.idRelease, req));
    }

    protected onStopAgent(): void {
        const idRun = this.release()?.idRunNotes;
        if (idRun === null || idRun === undefined || this.isBusy()) {
            return;
        }
        this.isBusy.set(true);
        this.agentRunApi
            .cancel$(idRun)
            .pipe(switchMap(() => this.api.loadOne$(this.idProject, this.idRelease)))
            .subscribe({
                next: release => {
                    this.isBusy.set(false);
                    this.applyRelease(release);
                },
                error: () => this.isBusy.set(false)
            });
    }

    protected onDismissRunError(): void {
        this.idRunErrorDismissed.set(this.release()?.idRunNotes ?? null);
    }

    protected onPublish(): void {
        this.runAfterSave(() => this.api.publish$(this.idProject, this.idRelease));
    }

    protected onEditNotes(): void {
        this.isEditingNotes.set(true);
    }

    protected onCancelEditNotes(): void {
        this.isEditingNotes.set(false);
    }

    protected onSaveNotes(notes: string): void {
        if (this.isBusy()) {
            return;
        }
        this.isBusy.set(true);
        this.api.updateNotes$(this.idProject, this.idRelease, notes).subscribe({
            next: release => {
                this.isBusy.set(false);
                this.isEditingNotes.set(false);
                this.applyRelease(release);
            },
            error: () => this.isBusy.set(false)
        });
    }

    protected onDelete(): void {
        this.isDeleteOpen.set(true);
    }

    protected onDeleteClosed(): void {
        this.isDeleteOpen.set(false);
    }

    protected onDeleted(): void {
        void this.router.navigate(['/project', this.idProject, 'release']);
    }

    private load(): void {
        this.api
            .loadOne$(this.idProject, this.idRelease)
            .subscribe(release => this.applyRelease(release));
    }

    private applyRelease(release: ReleaseView): void {
        const previous = this.release();
        this.release.set(release);
        if (release.status !== ReleaseStatus.Draft) {
            this.loadTasks(release);
            return;
        }
        const server = this.toReq(release);
        if (!previous) {
            this.form.setValue(server, { emitEvent: false });
            this.lastSaved = server;
            this.formValue.set(this.form.getRawValue());
            this.notesMode.set(
                release.notes.trim() ? ReleaseNotesMode.Editor : ReleaseNotesMode.Choose
            );
            this.loadDraft(release);
            return;
        }
        if (release.notes !== previous.notes && release.notes !== this.form.controls.notes.value) {
            this.form.controls.notes.setValue(release.notes, { emitEvent: false });
            this.editor()?.replaceValue(release.notes);
            this.lastSaved = { ...(this.lastSaved ?? server), notes: release.notes };
            this.formValue.set(this.form.getRawValue());
        }
        if (release.isNotesPending !== previous.isNotesPending) {
            this.notesMode.set(
                release.notes.trim() ? ReleaseNotesMode.Editor : ReleaseNotesMode.Choose
            );
        }
    }

    private onStepEntered(step: ReleaseStep): void {
        void this.router.navigate([], {
            relativeTo: this.route,
            queryParams: { step },
            replaceUrl: true
        });
        const release = this.release();
        if (release?.status !== ReleaseStatus.Draft) {
            return;
        }
        if (step !== ReleaseStep.Version) {
            this.loadDraft(release);
        }
    }

    private loadDraft(release: ReleaseView): void {
        if (release.idTag === null) {
            return;
        }
        this.api
            .loadDraft$(this.idProject, release.idTag)
            .subscribe(draft => this.draft.set(draft));
    }

    private bindRun(idRun: number | null): void {
        this.thinkingStore.bind(idRun);
        if (idRun === null) {
            this.notesRun.set(null);
            return;
        }
        if (this.notesRun()?.idRun === idRun) {
            return;
        }
        this.agentRunApi.loadOne$(idRun).subscribe(run => this.applyRun(run));
    }

    private loadTasks(release: ReleaseView): void {
        if (release.idTag === null) {
            this.tasks.set([]);
            return;
        }
        this.taskService
            .loadRows$(release.idProject, release.idTag)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(rows => this.tasks.set(rows));
    }

    private applyRun(run: AgentRun | null): void {
        if (run && run.idRun === this.release()?.idRunNotes) {
            this.notesRun.set(run);
        }
    }

    private runAfterSave(action: () => Observable<ReleaseView>): void {
        if (this.isBusy()) {
            return;
        }
        this.isBusy.set(true);
        this.save$()
            .pipe(
                switchMap(isSaved => (isSaved ? action() : EMPTY)),
                catchError(() => EMPTY),
                finalize(() => this.isBusy.set(false))
            )
            .subscribe(release => this.applyRelease(release));
    }

    private save$(): Observable<boolean> {
        if (!this.canEdit() || this.form.invalid || !this.isDirty()) {
            return of(true);
        }
        const value = this.form.getRawValue();
        value.name = value.name.trim();
        const { notes, ...settings } = value;
        const body: UpdateReleaseReq = notes === this.lastSaved?.notes ? settings : value;
        this.saveState.set(UiSaveState.Saving);
        return this.api.update$(this.idProject, this.idRelease, body).pipe(
            tap(release => {
                this.lastSaved = value;
                this.saveState.set(UiSaveState.Saved);
                this.release.set(release);
            }),
            map(() => true),
            catchError(() => {
                this.saveState.set(UiSaveState.Error);
                return of(false);
            })
        );
    }

    private isDirty(): boolean {
        const value = this.form.getRawValue();
        return !isEqual({ ...value, name: value.name.trim() }, this.lastSaved);
    }

    private toReq(release: ReleaseView): Required<UpdateReleaseReq> {
        return {
            name: release.name,
            recreateTag: release.recreateTag,
            openIssues: release.openIssues,
            notes: release.notes
        };
    }
}
