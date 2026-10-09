import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    OnDestroy,
    WritableSignal,
    computed,
    inject,
    signal,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { User } from 'src/app/auth/model/user.model';
import { ActivatedRoute, Router } from '@angular/router';
import { Observable, Subject, filter, interval, map, of, take } from 'rxjs';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { IssueApi } from 'src/app/issue/api/issue.api.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { ApiError } from 'src/app/shared/model/api-error.model';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { WikiApi } from '../../api/wiki.api.service';
import {
    WikiEditorComponent,
    WikiEditorIssueOption,
    WikiEditorPageOption
} from '../../components/wiki-editor/wiki-editor.component';
import { WikiAgentAccess } from '../../constants/wiki-agent-access.enum';
import { WikiMergeChunkKind } from '../../constants/wiki-merge-chunk-kind.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiBudgetConverter } from '../../converter/wiki-budget.converter';
import { WikiCalloutConverter } from '../../converter/wiki-callout.converter';
import { WikiLinkConverter } from '../../converter/wiki-link.converter';
import { WikiMergeConverter } from '../../converter/wiki-merge.converter';
import { WikiTreeConverter } from '../../converter/wiki-tree.converter';
import {
    WikiEditor,
    WikiPageDraft,
    WikiPageView,
    WikiResolvedLink,
    WikiSavedNotice
} from '../../model/wiki-page.model';
import { WikiTreeNode } from '../../model/wiki-tree.model';
import { WikiProposalAction } from '../../constants/wiki-proposal-action.enum';
import { WikiProposalState } from '../../constants/wiki-proposal-state.enum';
import { WikiProposalConverter } from '../../converter/wiki-proposal.converter';
import { WikiProposal } from '../../model/wiki-proposal.model';
import { WikiConflict, WikiMergeChunk, WikiMergeResult } from '../../model/wiki-save.model';
import { WikiParentOption } from '../../entity/wiki-parent-option.entity';
import { WikiTreeStore } from '../../store/wiki-tree.store';

interface WikiConflictState {
    chunks: WikiMergeChunk[];
    currentVersion: number;
    theirsName: string;
}

enum WikiDraftState {
    Idle = 'idle',
    Saving = 'saving',
    Saved = 'saved'
}

const DRAFT_INTERVAL_MS = 5000;
const HEARTBEAT_INTERVAL_MS = 15000;

@Component({
    selector: 'app-wiki-edit',
    templateUrl: './wiki-edit.page.html',
    styleUrls: ['./wiki-edit.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    host: { '(window:beforeunload)': 'onBeforeUnload($event)' }
})
export class WikiEditPage implements OnDestroy {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly api = inject(WikiApi);
    private readonly issueApi = inject(IssueApi);
    private readonly toast = inject(ToastNotificationService);
    private readonly authStore = inject(AuthStore);
    private readonly destroyRef = inject(DestroyRef);
    private readonly i18n = inject(I18nService);
    protected readonly store = inject(WikiTreeStore);

    private readonly calloutLabels = WikiCalloutConverter.toLabels(key => this.i18n.instant(key));

    protected readonly AgentAccess = WikiAgentAccess;
    protected readonly SpaceKind = WikiSpaceKind;
    protected readonly DraftState = WikiDraftState;

    private readonly editor = viewChild<WikiEditorComponent>('editor');

    protected readonly isCreate = this.route.snapshot.data['isCreate'] === true;
    protected readonly idProject = Number(this.route.snapshot.paramMap.get('idProject'));
    protected readonly idProposal =
        Number(this.route.snapshot.queryParamMap.get('proposal')) || null;
    protected readonly proposal = signal<WikiProposal | null>(null);

    protected readonly view = signal<WikiPageView | null>(null);
    protected readonly isReady = signal(false);
    protected readonly space = signal<WikiSpaceKind>(WikiSpaceKind.Project);
    protected readonly idParent = signal<number | null>(null);
    protected readonly title = signal('');
    protected readonly summary = signal('');
    protected readonly body = signal('');
    protected readonly agentAccess = signal<WikiAgentAccess>(WikiAgentAccess.OnDemand);
    protected readonly note = signal('');
    protected readonly baseVersion = signal(1);
    protected readonly isSaving = signal(false);
    protected readonly draftState = signal<WikiDraftState>(WikiDraftState.Idle);
    protected readonly pendingDraft = signal<WikiPageDraft | null>(null);
    protected readonly editors = signal<WikiEditor[]>([]);
    protected readonly incoming = signal<WikiSavedNotice | null>(null);
    protected readonly conflict = signal<WikiConflictState | null>(null);
    protected readonly issueOptions = signal<WikiEditorIssueOption[]>([]);

    private lastDraft = '';
    private savedContent = '';
    private leaveDecision: Subject<boolean> | null = null;
    protected readonly isLeaveOpen = signal(false);
    private isIssuesRequested = false;

    private readonly usersMap = toSignal(inject(ProjectMemberStore).usersMap$, {
        initialValue: new Map<number, User>()
    });

    protected readonly spaceView = computed(() =>
        this.space() === WikiSpaceKind.Instance
            ? this.store.sharedSpace()
            : this.store.projectSpace()
    );

    protected readonly canManage = computed(
        () => this.view()?.canManage ?? !!this.spaceView()?.canManage
    );

    protected readonly canChooseShared = computed(() => !!this.store.sharedSpace()?.canEdit);

    protected readonly budgetTokens = computed(() => {
        const tree = this.store.tree();
        if (!tree) {
            return 0;
        }
        const own = Math.ceil((this.title().length + this.body().length) / 4);
        const wasAlways = this.view()?.page.agentAccess === WikiAgentAccess.Always;
        const previous = wasAlways
            ? Math.ceil(
                  ((this.view()?.page.title.length ?? 0) + (this.view()?.page.body.length ?? 0)) / 4
              )
            : 0;
        const isAlways = this.agentAccess() === WikiAgentAccess.Always;
        return tree.alwaysTokens - previous + (isAlways ? own : 0);
    });

    protected readonly budgetPercent = computed(() => {
        const limit = this.store.tree()?.tokenLimit ?? 0;
        return limit > 0 ? Math.min(100, Math.round((this.budgetTokens() / limit) * 100)) : 100;
    });

    protected readonly isOverBudget = computed(() => {
        const page = this.view()?.page;
        return WikiBudgetConverter.toIsOverBudget({
            isAlways: this.agentAccess() === WikiAgentAccess.Always,
            wasAlways: page?.agentAccess === WikiAgentAccess.Always,
            isShared: this.space() === WikiSpaceKind.Instance,
            ownChars: this.title().length + this.body().length,
            previousChars: page ? page.title.length + page.body.length : 0,
            usedTokens: this.budgetTokens(),
            limit: this.store.tree()?.tokenLimit ?? 0
        });
    });

    protected readonly parentOptions = computed<WikiParentOption[]>(() => {
        const tree = this.store.tree();
        const idSpace = this.spaceView()?.idSpace;
        if (!tree || idSpace === undefined) {
            return [];
        }
        return WikiTreeConverter.toParentOptions(
            tree.nodes,
            idSpace,
            this.i18n.instant('WIKI.MOVE.ROOT')
        );
    });

    protected readonly pageOptions = computed<WikiEditorPageOption[]>(() => {
        const tree = this.store.tree();
        const sharedId = this.store.sharedSpace()?.idSpace;
        return (tree?.nodes ?? []).map(node => ({
            title: node.title,
            slug: node.slug,
            isShared: node.idSpace === sharedId && this.space() === WikiSpaceKind.Project
        }));
    });

    protected readonly preview = computed(() =>
        WikiLinkConverter.toMarkdown(WikiCalloutConverter.toHtml(this.body(), this.calloutLabels), {
            idProject: this.idProject,
            links: this.previewLinks(),
            sharedIdSpace: this.store.sharedSpace()?.idSpace ?? null
        })
    );

    private readonly previewLinks = computed(() => {
        const nodes = this.store.tree()?.nodes ?? [];
        const sharedId = this.store.sharedSpace()?.idSpace ?? null;
        const ownId = this.spaceView()?.idSpace ?? null;
        const own = nodes.filter(node => node.idSpace === ownId);
        const ownSlugs = new Set(own.map(node => node.slug));
        const toLink = (node: WikiTreeNode, shared: boolean): WikiResolvedLink => ({
            shared,
            slug: node.slug,
            idPage: node.idPage,
            idSpace: node.idSpace,
            title: node.title
        });
        const sharedNodes = nodes.filter(node => node.idSpace === sharedId);
        return [
            ...own.map(node => toLink(node, ownId === sharedId)),
            ...sharedNodes.map(node => toLink(node, true)),
            ...sharedNodes.filter(node => !ownSlugs.has(node.slug)).map(node => toLink(node, false))
        ];
    });

    protected readonly editorNames = computed(() =>
        this.editors()
            .map(editor => editor.name)
            .join(', ')
    );

    protected readonly spaceOptions = computed(() => [
        { label: this.i18n.instant('WIKI.SPACE.PROJECT'), value: WikiSpaceKind.Project },
        { label: this.i18n.instant('WIKI.SPACE.INSTANCE'), value: WikiSpaceKind.Instance }
    ]);

    protected readonly incomingName = computed(() => this.incoming()?.userName ?? '');

    protected readonly saveLabel = computed(() => {
        if (this.idProposal) {
            const proposal = this.proposal();
            const action = proposal ? WikiProposalConverter.toAction(proposal.state) : null;
            return action === WikiProposalAction.Publish
                ? 'WIKI.PROPOSAL.PUBLISH'
                : 'WIKI.PROPOSAL.APPROVE';
        }
        return this.isCreate ? 'WIKI.SAVE.CREATE' : 'WIKI.SAVE.ACTION';
    });

    public constructor() {
        if (this.isCreate && this.idProposal) {
            this.initCreateFromProposal(this.idProposal);
        } else if (this.isCreate) {
            this.initCreate();
        } else {
            this.loadForEdit();
        }

        inject(NoticeService)
            .wikiPage$.pipe(
                map(notice => notice.payload),
                filter(
                    payload =>
                        payload.idPage === this.view()?.page.idPage &&
                        payload.versionNo > this.baseVersion() &&
                        payload.idUser !== this.authStore.user()?.idUser
                ),
                takeUntilDestroyed()
            )
            .subscribe(payload => this.incoming.set(payload));

        interval(DRAFT_INTERVAL_MS)
            .pipe(takeUntilDestroyed())
            .subscribe(() => this.saveDraft());

        interval(HEARTBEAT_INTERVAL_MS)
            .pipe(takeUntilDestroyed())
            .subscribe(() => this.heartbeat());
    }

    protected onBeforeUnload(event: BeforeUnloadEvent): void {
        const hasUnsavedDraft = !!this.view() && this.draftSignature() !== this.lastDraft;
        if (this.hasUnsavedNewPage() || hasUnsavedDraft) {
            event.preventDefault();
        }
    }

    public confirmLeave$(): Observable<boolean> {
        if (!this.hasUnsavedNewPage()) {
            return of(true);
        }
        this.leaveDecision?.next(false);
        this.leaveDecision = new Subject<boolean>();
        this.isLeaveOpen.set(true);
        return this.leaveDecision.pipe(take(1));
    }

    protected onLeaveDecided(isLeaving: boolean): void {
        this.isLeaveOpen.set(false);
        this.leaveDecision?.next(isLeaving);
        this.leaveDecision = null;
    }

    public ngOnDestroy(): void {
        this.saveDraft();
        const view = this.view();
        if (view) {
            this.api.leave$(view.page.idPage).subscribe({ error: () => undefined });
        }
    }

    protected onTextInput(event: Event, field: WritableSignal<string>): void {
        if (
            event.target instanceof HTMLInputElement ||
            event.target instanceof HTMLTextAreaElement
        ) {
            field.set(event.target.value);
        }
    }

    protected onSpaceChange(space: WikiSpaceKind): void {
        this.space.set(space);
        this.idParent.set(null);
    }

    protected onConflictCancelled(): void {
        this.conflict.set(null);
    }

    protected onBodyChange(body: string): void {
        this.body.set(body);
    }

    protected onIssuesRequested(): void {
        if (this.isIssuesRequested) {
            return;
        }
        this.isIssuesRequested = true;
        this.issueApi
            .load$({ idProject: this.idProject, orderColumn: 'createAt', orderDirection: 'desc' })
            .subscribe(issues =>
                this.issueOptions.set(
                    issues.map(issue => ({
                        idIssuePublic: issue.idIssuePublic,
                        title: issue.title
                    }))
                )
            );
    }

    protected onContinueDraft(): void {
        const draft = this.pendingDraft();
        const view = this.view();
        this.pendingDraft.set(null);
        if (!draft || !view) {
            return;
        }
        this.title.set(draft.title || view.page.title);
        this.summary.set(draft.summary);
        this.applyBody(draft.body);
        this.baseVersion.set(draft.baseVersion);
        if (draft.baseVersion < view.page.versionNo) {
            this.onApplyIncoming();
        }
    }

    protected onDiscardDraft(): void {
        const view = this.view();
        this.pendingDraft.set(null);
        if (view) {
            this.api.deleteDraft$(view.page.idPage).subscribe();
        }
    }

    protected onApplyIncoming(): void {
        const view = this.view();
        if (!view) {
            return;
        }
        this.api
            .mergePreview$(view.page.idPage, {
                baseVersion: this.baseVersion(),
                title: this.title(),
                summary: this.summary(),
                body: this.body(),
                agentAccess: this.agentAccess()
            })
            .subscribe(preview => {
                const theirsName = this.theirsLabel(null);
                this.incoming.set(null);
                this.applyMergedFields(preview);
                if (preview.merge.conflicts > 0) {
                    this.conflict.set({
                        chunks: preview.merge.chunks,
                        currentVersion: preview.currentVersion,
                        theirsName
                    });
                    return;
                }
                this.applyBody(this.stableText(preview.merge));
                this.baseVersion.set(preview.currentVersion);
                this.toast.showInfo('WIKI.LIVE.APPLIED');
            });
    }

    protected onResolved(text: string): void {
        const conflict = this.conflict();
        if (!conflict) {
            return;
        }
        this.applyBody(text);
        this.baseVersion.set(conflict.currentVersion);
        this.conflict.set(null);
        this.onSave();
    }

    protected onCancel(): void {
        const view = this.view();
        this.lastDraft = this.draftSignature();
        this.savedContent = this.contentSignature();
        if (this.idProposal) {
            void this.router.navigate([
                '/project',
                this.idProject,
                'wiki',
                'proposals',
                this.idProposal
            ]);
            return;
        }
        if (view) {
            this.api.deleteDraft$(view.page.idPage).subscribe();
            void this.router.navigate([
                '/project',
                this.idProject,
                'wiki',
                view.spaceKind,
                view.page.slug
            ]);
            return;
        }
        void this.router.navigate(['/project', this.idProject, 'wiki']);
    }

    protected onSave(): void {
        if (!this.title().trim() || this.isSaving()) {
            return;
        }
        this.isSaving.set(true);
        if (this.idProposal) {
            this.acceptProposal(this.idProposal);
            return;
        }
        if (this.isCreate) {
            this.create();
            return;
        }
        const view = this.view();
        if (!view) {
            return;
        }
        this.api
            .update$(view.page.idPage, {
                baseVersion: this.baseVersion(),
                title: this.title(),
                summary: this.summary(),
                body: this.body(),
                agentAccess: this.agentAccess(),
                note: this.note()
            })
            .subscribe({
                next: result => {
                    this.isSaving.set(false);
                    this.lastDraft = this.draftSignature();
                    this.toast.showSuccess(
                        result.mergedFrom ? 'WIKI.SAVE.MERGED' : 'WIKI.SAVE.DONE'
                    );
                    this.store.reload();
                    void this.router.navigate([
                        '/project',
                        this.idProject,
                        'wiki',
                        view.spaceKind,
                        result.page.slug
                    ]);
                },
                error: (error: unknown) => this.onSaveFailed(error)
            });
    }

    private acceptProposal(idProposal: number): void {
        this.api
            .acceptProposal$(idProposal, {
                baseVersion: this.isCreate ? undefined : this.baseVersion(),
                title: this.title(),
                summary: this.summary(),
                body: this.body(),
                agentAccess: this.agentAccess(),
                note: this.note()
            })
            .subscribe({
                next: result => {
                    this.isSaving.set(false);
                    this.savedContent = this.contentSignature();
                    this.lastDraft = this.draftSignature();
                    this.store.reload();
                    const proposal = result.proposal;
                    if (proposal.state === WikiProposalState.Approved) {
                        this.toast.showSuccess('WIKI.PROPOSAL.APPROVED');
                        void this.router.navigate([
                            '/project',
                            this.idProject,
                            'wiki',
                            'proposals',
                            idProposal
                        ]);
                        return;
                    }
                    this.toast.showSuccess('WIKI.PROPOSAL.ACCEPTED');
                    void this.router.navigate([
                        '/project',
                        this.idProject,
                        'wiki',
                        proposal.spaceKind,
                        result.page?.slug ?? proposal.slug
                    ]);
                },
                error: (error: unknown) => this.onSaveFailed(error)
            });
    }

    private onSaveFailed(error: unknown): void {
        this.isSaving.set(false);
        const conflict = WikiMergeConverter.toConflict(error);
        if (conflict) {
            this.applyMergedFields(conflict);
            this.conflict.set({
                chunks: conflict.merge.chunks,
                currentVersion: conflict.current.versionNo,
                theirsName: this.theirsLabel(conflict.current.updateBy)
            });
            return;
        }
        this.toast.showError(ApiError.translateKeyOf(error) ?? 'error.internal');
    }

    private initCreateFromProposal(idProposal: number): void {
        this.api
            .loadProposal$(idProposal)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(detail => {
                const proposal = detail.proposal;
                this.proposal.set(proposal);
                this.space.set(proposal.spaceKind);
                this.idParent.set(proposal.idParent);
                this.title.set(proposal.title);
                this.summary.set(proposal.summary);
                this.body.set(proposal.body ?? '');
                this.savedContent = this.contentSignature();
                this.isReady.set(true);
            });
    }

    private applyProposal(view: WikiPageView, idProposal: number): void {
        this.api
            .loadProposal$(idProposal)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(detail => {
                const proposal = detail.proposal;
                this.proposal.set(proposal);
                this.pendingDraft.set(null);
                this.title.set(proposal.title);
                this.summary.set(proposal.summary);
                this.applyBody(proposal.body ?? '');
                this.baseVersion.set(proposal.baseVersion ?? view.page.versionNo);
                this.isReady.set(true);
                if (this.baseVersion() < view.page.versionNo) {
                    this.onApplyIncoming();
                }
            });
    }

    private create(): void {
        this.api
            .insert$(this.idProject, {
                space: this.space(),
                idParent: this.idParent(),
                title: this.title(),
                summary: this.summary(),
                body: this.body(),
                agentAccess: this.agentAccess(),
                note: this.note()
            })
            .subscribe({
                next: page => {
                    this.isSaving.set(false);
                    this.savedContent = this.contentSignature();
                    this.toast.showSuccess('WIKI.SAVE.CREATED');
                    this.store.reload();
                    void this.router.navigate([
                        '/project',
                        this.idProject,
                        'wiki',
                        this.space(),
                        page.slug
                    ]);
                },
                error: () => this.isSaving.set(false)
            });
    }

    private initCreate(): void {
        const query = this.route.snapshot.queryParamMap;
        this.space.set(
            query.get('space') === WikiSpaceKind.Instance
                ? WikiSpaceKind.Instance
                : WikiSpaceKind.Project
        );
        this.title.set(query.get('title') ?? '');
        const idParent = Number(query.get('parent'));
        this.idParent.set(Number.isFinite(idParent) && idParent > 0 ? idParent : null);
        this.savedContent = this.contentSignature();
        this.isReady.set(true);
    }

    private loadForEdit(): void {
        const params = this.route.snapshot.paramMap;
        const space = params.get('space') as WikiSpaceKind;
        this.api
            .loadOne$(this.idProject, space, params.get('slug') ?? '')
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(view => {
                this.view.set(view);
                this.space.set(view.spaceKind);
                this.title.set(view.page.title);
                this.summary.set(view.page.summary);
                this.body.set(view.page.body);
                this.agentAccess.set(view.page.agentAccess);
                this.baseVersion.set(view.page.versionNo);
                this.lastDraft = this.draftSignature();
                this.heartbeat();
                if (this.idProposal) {
                    this.applyProposal(view, this.idProposal);
                    return;
                }
                this.pendingDraft.set(view.draft);
                this.isReady.set(true);
            });
    }

    private heartbeat(): void {
        const view = this.view();
        if (!view) {
            return;
        }
        this.api.heartbeat$(view.page.idPage).subscribe({
            next: editors => this.editors.set(editors),
            error: () => undefined
        });
    }

    private saveDraft(): void {
        const view = this.view();
        const signature = this.draftSignature();
        if (
            !view ||
            this.idProposal ||
            this.pendingDraft() ||
            this.isSaving() ||
            signature === this.lastDraft
        ) {
            return;
        }
        this.draftState.set(WikiDraftState.Saving);
        this.api
            .saveDraft$(view.page.idPage, {
                baseVersion: this.baseVersion(),
                title: this.title(),
                summary: this.summary(),
                body: this.body()
            })
            .subscribe({
                next: () => {
                    this.lastDraft = signature;
                    this.draftState.set(WikiDraftState.Saved);
                },
                error: () => this.draftState.set(WikiDraftState.Idle)
            });
    }

    private hasUnsavedNewPage(): boolean {
        return this.isCreate && this.isReady() && this.contentSignature() !== this.savedContent;
    }

    private contentSignature(): string {
        return JSON.stringify([this.title(), this.summary(), this.body(), this.note()]);
    }

    private draftSignature(): string {
        return JSON.stringify([this.baseVersion(), this.title(), this.summary(), this.body()]);
    }

    private applyBody(text: string): void {
        this.body.set(text);
        this.editor()?.replaceValue(text);
    }

    private stableText(merge: WikiMergeResult): string {
        return merge.chunks
            .filter(chunk => chunk.kind === WikiMergeChunkKind.Stable)
            .flatMap(chunk => chunk.lines ?? [])
            .join('\n');
    }

    private theirsLabel(idUser: number | null): string {
        const name = idUser ? this.usersMap().get(idUser)?.name : this.incomingName();
        return name || '—';
    }

    private applyMergedFields(
        fields: Pick<WikiConflict, 'title' | 'summary' | 'agentAccess'>
    ): void {
        this.title.set(fields.title);
        this.summary.set(fields.summary);
        this.agentAccess.set(fields.agentAccess);
    }
}
