import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    OnInit,
    computed,
    effect,
    inject,
    signal,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, ParamMap, Router } from '@angular/router';
import { combineLatest, Subject } from 'rxjs';
import { finalize, switchMap } from 'rxjs/operators';
import { AgentThinkingEvent, AgentThinkingNotice } from 'src/app/agent/model/agent-thinking.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { ProjectContextStore } from 'src/app/home/store/project-context.store';
import { LinkScope } from 'src/app/link/service/link-scope.service';
import { MessageEditorComponent } from 'src/app/message/components/message-editor/message-editor.component';
import { ProjectApi } from 'src/app/project/api/project.api.service';
import { Project } from 'src/app/project/model/project.model';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';
import { ApiError } from 'src/app/shared/model/api-error.model';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { AgentConversationApi } from '../../api/agent-conversation.api.service';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { AgentConversationAuthor } from '../../constants/agent-conversation-author.enum';
import { AgentConversationMessageKind } from '../../constants/agent-conversation-message-kind.enum';
import { AgentStatusState } from '../../constants/agent-status-state.enum';
import { AgentChatEntry } from '../../entity/agent-chat-entry.entity';
import { AgentConversationMessageConverter } from '../../entity/agent-conversation-message.converter';
import { LiveThinking } from '../../entity/live-thinking.entity';
import { StoredThinkingStatus } from '../../constants/stored-thinking-status.enum';
import {
    AgentConversation,
    AgentConversationDetail,
    AgentConversationListItem,
    AgentConversationMessage,
    AgentConversationSettings,
    AgentConversationTurnNotice
} from '../../model/agent-conversation.model';
import { AgentConversationUnreadStore } from '../../store/agent-conversation-unread.store';

const CONTINUATION_MS = 5 * 60 * 1000;

/**
 * `/agent-chat` lists the owner's conversations, `?new=1` starts one and
 * `/agent-chat/:idConversation` opens one. The list holds every conversation; `?idProject=`, or
 * else the side menu's project, picks the project of a new one and puts its conversations first.
 */
@Component({
    selector: 'app-agent-chat',
    templateUrl: './agent-chat.page.html',
    styleUrls: ['./agent-chat.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    providers: [LinkScope]
})
export class AgentChatPage implements OnInit {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly api = inject(AgentConversationApi);
    private readonly projectApi = inject(ProjectApi);
    private readonly authStore = inject(AuthStore);
    private readonly sNotice = inject(NoticeService);
    private readonly i18n = inject(I18nService);
    private readonly toast = inject(UiToastService);
    private readonly unreadStore = inject(AgentConversationUnreadStore);
    private readonly linkScope = inject(LinkScope);
    private readonly projectContext = inject(ProjectContextStore);
    private readonly destroyRef = inject(DestroyRef);

    private readonly msgList = viewChild<ElementRef<HTMLElement>>('msgList');
    private readonly composer = viewChild<MessageEditorComponent>('composer');

    protected readonly idProject = signal<number | null>(null);
    protected readonly isNew = signal(false);
    protected readonly idConversation = signal<number | null>(null);
    protected readonly AgentConversationAuthor = AgentConversationAuthor;
    protected readonly AgentConversationMessageKind = AgentConversationMessageKind;
    protected readonly conversations = signal<AgentConversationListItem[]>([]);
    /** Only the first load: later reloads keep the list in view. */
    protected readonly isListLoading = signal(true);
    protected readonly projects = signal<Project[]>([]);
    protected readonly detail = signal<AgentConversationDetail | null>(null);
    protected readonly idProjectOpen = computed(
        () => this.idProject() ?? this.projectContext.idProject()
    );
    protected readonly projectName = computed(
        () => this.projects().find(p => p.idProject === this.idProjectOpen())?.name ?? null
    );
    protected readonly sendError = signal<string | null>(null);
    protected readonly projectError = signal<string | null>(null);
    /** The task draft the owner is editing into an issue. */
    protected readonly draftBody = signal<string | null>(null);

    private readonly messages = signal<AgentConversationMessage[]>([]);
    // The task of the running turn as the first thinking notice named it, before
    // the status has caught up.
    private readonly idTaskStreaming = signal<number | null>(null);
    private readonly isRetrying = signal(false);
    // Ids whose DELETE was sent and not refused; notices about them are dropped.
    private readonly deletedIds = new Set<number>();
    private readonly isDraftRequesting = signal(false);
    private readonly owner = this.authStore.getUser();
    // The thinking of each turn, as it streamed while the agent answered or as fetched on first open.
    private readonly thinkingByTask = signal<ReadonlyMap<number, AgentThinkingEvent[]>>(new Map());
    private readonly idsThinkingOpen = signal<ReadonlySet<number>>(new Set());
    // Turns whose stored thinking is loading, or came back empty or failed; neither is kept, so the
    // next open asks again.
    private readonly thinkingStatusByTask = signal<
        ReadonlyMap<number, Exclude<StoredThinkingStatus, StoredThinkingStatus.Ready>>
    >(new Map());

    protected readonly entries = computed<AgentChatEntry[]>(() => {
        const detail = this.detail();
        return detail
            ? AgentConversationMessageConverter.toEntries(this.messages(), this.owner, detail.agent)
            : [];
    });

    // The owner's own turn is the only busy state with the chat stage.
    protected readonly isAnswering = computed(() => {
        const status = this.detail()?.agentStatus;
        return status?.state === AgentStatusState.Busy && status.stage === AgentStage.Chat;
    });

    // The running turn's task, so the live block replays it from the first batch.
    protected readonly idTaskLive = computed(
        () => this.detail()?.agentStatus.idTask ?? this.idTaskStreaming()
    );

    private readonly isTurnUnderway = computed(
        () => this.detail()?.agentStatus.state === AgentStatusState.Queued || this.isAnswering()
    );

    // Only the newest line can be retried, and only while no turn is under way.
    protected readonly idMessageRetryable = computed<number | null>(() => {
        const last = this.entries().at(-1);
        return last?.kind === AgentConversationMessageKind.Error &&
            last.author === AgentConversationAuthor.System &&
            !this.isTurnUnderway() &&
            !this.isRetrying()
            ? last.idMessage
            : null;
    });

    // Context projects without a main one cannot be read, so the owner picks the main project first.
    protected readonly isMainRequired = computed(() => {
        const projects = this.detail()?.projects ?? [];
        return projects.length > 0 && !projects.some(project => project.isMain);
    });

    // Waits while a turn is under way or a draft is due.
    protected readonly canCreateTask = computed(() => {
        const conversation = this.detail()?.conversation;
        return (
            !!conversation &&
            conversation.idRun !== null &&
            conversation.taskDraftRequest === null &&
            !this.isMainRequired() &&
            !this.isTurnUnderway() &&
            !this.isDraftRequesting()
        );
    });

    private readonly listRequest = new Subject<void>();
    private pendingSeq = 0;
    private isStatusReloading = false;

    public constructor() {
        effect(() => {
            if (this.entries().length > 0 || this.isAnswering()) {
                this.scrollToBottom();
            }
        });
    }

    public ngOnInit(): void {
        this.listRequest
            .pipe(
                switchMap(() => this.api.load$()),
                takeUntilDestroyed(this.destroyRef)
            )
            .subscribe({
                next: list => {
                    this.conversations.set(list);
                    this.isListLoading.set(false);
                },
                error: () => this.isListLoading.set(false)
            });
        this.listRequest.next();

        this.projectApi.load$().subscribe(projects => this.projects.set(projects));

        combineLatest([this.route.paramMap, this.route.queryParamMap])
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(([params, query]) => this.onRoute(params, query));

        this.sNotice.agentConversation$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(notice => {
                if (notice.action === NoticeAction.Create) {
                    this.onConversationMessage(notice.payload);
                }
            });

        this.sNotice.agentConversationTurn$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(notice => this.onTurnStarted(notice.payload));

        this.sNotice.agentThinking$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(notice => this.onThinking(notice.payload as AgentThinkingNotice | null));
    }

    protected onBack(): void {
        void this.router.navigate(['/agent-chat'], {
            queryParams: { idProject: this.idProject() }
        });
    }

    protected onCreated(conversation: AgentConversation): void {
        this.unreadStore.notifyChanged();
        this.listRequest.next();
        void this.router.navigate(['/agent-chat', conversation.idConversation], {
            queryParams: { idProject: this.idProject() }
        });
    }

    protected onAttachRefused(projects: Project[]): void {
        const names = projects.map(project => project.name).join(', ');
        this.toast.show({
            severity: 'error',
            detail: `${this.i18n.instant('AGENT_CHAT.NEW_FORM.PROJECTS_REFUSED', { names })} ${this.i18n.instant('error.agent_not_in_project')}`
        });
    }

    protected onAttachFailed(projects: Project[]): void {
        const names = projects.map(project => project.name).join(', ');
        this.toast.show({
            severity: 'error',
            detail: this.i18n.instant('AGENT_CHAT.NEW_FORM.PROJECTS_FAILED', { names })
        });
    }

    // The message shows at once; the server copy replaces it, or a refusal removes it and puts the text back.
    protected onSend(text: string): void {
        const idConversation = this.idConversation();
        if (idConversation === null) {
            return;
        }
        this.sendError.set(null);
        const pending: AgentConversationMessage = {
            idMessage: -++this.pendingSeq,
            idConversation,
            author: AgentConversationAuthor.User,
            kind: AgentConversationMessageKind.Text,
            body: text,
            idUser: this.owner.idUser,
            idTask: null,
            hasThinking: false,
            createdAt: new Date().toISOString()
        };
        this.messages.update(list => [...list, pending]);
        this.api.insertMessage$(idConversation, text).subscribe({
            next: saved => {
                this.upsertMessage(saved);
                this.reloadDetail();
                // The first message titles the conversation.
                this.listRequest.next();
                this.unreadStore.notifyChanged();
            },
            error: (err: unknown) => {
                this.messages.update(list => list.filter(m => m.idMessage !== pending.idMessage));
                if (this.idConversation() === idConversation) {
                    this.sendError.set(ApiError.translateKeyOf(err) ?? 'error.internal');
                    this.composer()?.writeValue(text);
                }
            }
        });
    }

    protected onAttachProject(idProject: number): void {
        const idConversation = this.idConversation();
        if (idConversation === null) {
            return;
        }
        this.projectError.set(null);
        this.api.insertProject$(idConversation, idProject).subscribe({
            next: () => {
                this.reloadDetail();
                this.listRequest.next();
            },
            error: (err: unknown) =>
                this.projectError.set(ApiError.translateKeyOf(err) ?? 'error.internal')
        });
    }

    protected onDetachProject(idProject: number): void {
        const idConversation = this.idConversation();
        if (idConversation === null) {
            return;
        }
        this.projectError.set(null);
        this.api.deleteProject$(idConversation, idProject).subscribe(() => {
            this.reloadDetail();
            this.listRequest.next();
        });
    }

    // The list and the top menu reorder; the header shows the new pin state.
    protected onPinConversation(idConversation: number, isPinned: boolean): void {
        this.api.setPinned$(idConversation, isPinned).subscribe(() => {
            this.listRequest.next();
            this.unreadStore.notifyChanged();
            if (this.idConversation() === idConversation) {
                this.reloadDetail();
            }
        });
    }

    // The header shows what was stored; a refusal (the agent started answering) shows the current model again.
    protected onChangeSettings(idConversation: number, settings: AgentConversationSettings): void {
        this.api.updateSettings$(idConversation, settings).subscribe({
            next: () => this.reloadDetail(),
            error: () => this.reloadDetail()
        });
    }

    // The list and the top menu drop it; the open one is left for the empty page.
    protected onDeleteConversation(idConversation: number): void {
        if (this.deletedIds.has(idConversation)) {
            return;
        }
        this.deletedIds.add(idConversation);
        // A refusal is already toasted by the error interceptor.
        this.api.delete$(idConversation).subscribe({
            next: () => {
                this.listRequest.next();
                this.unreadStore.notifyChanged();
                if (this.idConversation() === idConversation) {
                    void this.router.navigate(['/agent-chat'], {
                        queryParams: { idProject: this.idProject() }
                    });
                }
            },
            error: () => {
                this.deletedIds.delete(idConversation);
                // Notices dropped while the delete was pending left the detail stale.
                if (this.idConversation() === idConversation) {
                    this.reloadDetail();
                }
            }
        });
    }

    // Retry stays hidden until the status shows the queued turn, so a second
    // click cannot be refused as a conflict.
    protected onRetry(): void {
        const idConversation = this.idConversation();
        if (idConversation === null || this.isRetrying()) {
            return;
        }
        this.sendError.set(null);
        this.isRetrying.set(true);
        this.api
            .retry$(idConversation)
            .pipe(
                switchMap(() => this.api.loadOne$(idConversation)),
                finalize(() => this.isRetrying.set(false))
            )
            .subscribe({
                next: detail => this.applyDetailOf(idConversation, detail),
                error: (err: unknown) =>
                    this.sendError.set(ApiError.translateKeyOf(err) ?? 'error.internal')
            });
    }

    // The draft arrives as an agent message; no issue run starts.
    protected onCreateTask(idMessage: number): void {
        const idConversation = this.idConversation();
        if (idConversation === null || !this.canCreateTask()) {
            return;
        }
        this.isDraftRequesting.set(true);
        this.api
            .insertTaskDraft$(idConversation, idMessage)
            .pipe(
                switchMap(() => this.api.loadOne$(idConversation)),
                finalize(() => this.isDraftRequesting.set(false))
            )
            .subscribe(detail => this.applyDetailOf(idConversation, detail));
    }

    protected onEditDraft(body: string): void {
        this.draftBody.set(body);
    }

    protected onDraftCancelled(): void {
        this.draftBody.set(null);
    }

    // A turn has thinking when the server says so or when it streamed some into this page.
    protected hasThinking(entry: AgentChatEntry): boolean {
        return (
            entry.idTask !== null &&
            (entry.hasThinking || (this.thinkingByTask().get(entry.idTask)?.length ?? 0) > 0)
        );
    }

    protected isThinkingOpen(entry: AgentChatEntry): boolean {
        return this.idsThinkingOpen().has(entry.idMessage);
    }

    protected thinkingOf(entry: AgentChatEntry): AgentThinkingEvent[] {
        return entry.idTask === null ? [] : (this.thinkingByTask().get(entry.idTask) ?? []);
    }

    protected thinkingStatusOf(entry: AgentChatEntry): StoredThinkingStatus {
        if (entry.idTask === null || this.thinkingByTask().has(entry.idTask)) {
            return StoredThinkingStatus.Ready;
        }
        return this.thinkingStatusByTask().get(entry.idTask) ?? StoredThinkingStatus.Loading;
    }

    protected onToggleThinking(entry: AgentChatEntry): void {
        const willOpen = !this.isThinkingOpen(entry);
        this.idsThinkingOpen.update(ids => {
            const next = new Set(ids);
            if (willOpen) {
                next.add(entry.idMessage);
            } else {
                next.delete(entry.idMessage);
            }
            return next;
        });
        if (willOpen && entry.idTask !== null) {
            this.loadThinking(entry.idTask);
        }
    }

    // A turn that lost some of its stream is fetched whole when its reply is opened.
    protected onLiveThinking(live: LiveThinking): void {
        this.thinkingByTask.update(byTask => {
            const next = new Map(byTask);
            if (live.hasGap) {
                next.delete(live.idTask);
            } else {
                next.set(live.idTask, live.events);
            }
            return next;
        });
    }

    protected isContinuation(index: number): boolean {
        const entries = this.entries();
        const current = entries[index].message;
        const previous = index > 0 ? entries[index - 1].message : null;
        if (!current || !previous) {
            return false;
        }
        return (
            previous.creator.idUser === current.creator.idUser &&
            current.createdAt.getTime() - previous.createdAt.getTime() < CONTINUATION_MS
        );
    }

    private onRoute(params: ParamMap, query: ParamMap): void {
        this.idProject.set(AgentChatPage.toId(query.get('idProject')));
        this.isNew.set(query.get('new') === '1');

        const idConversation = AgentChatPage.toId(params.get('idConversation'));
        if (idConversation !== this.idConversation()) {
            this.open(idConversation);
        }
    }

    private open(idConversation: number | null): void {
        this.idConversation.set(idConversation);
        this.detail.set(null);
        this.messages.set([]);
        this.idTaskStreaming.set(null);
        this.sendError.set(null);
        this.projectError.set(null);
        this.draftBody.set(null);
        this.thinkingByTask.set(new Map());
        this.idsThinkingOpen.set(new Set());
        this.thinkingStatusByTask.set(new Map());
        this.linkScope.idProject.set(null);
        if (idConversation === null) {
            return;
        }
        this.api.loadOne$(idConversation).subscribe(detail => {
            if (this.idConversation() !== idConversation) {
                return;
            }
            this.applyDetail(detail);
            if (AgentChatPage.isUnread(detail.conversation)) {
                this.markRead(idConversation);
            }
        });
        this.api.loadMessages$(idConversation).subscribe(loaded => {
            if (this.idConversation() !== idConversation) {
                return;
            }
            // Keep what was sent or streamed in while the history loaded.
            const ids = new Set(loaded.map(m => m.idMessage));
            this.messages.update(list => [...loaded, ...list.filter(m => !ids.has(m.idMessage))]);
        });
    }

    private onConversationMessage(message: AgentConversationMessage): void {
        this.listRequest.next();
        if (
            message.idConversation !== this.idConversation() ||
            this.deletedIds.has(message.idConversation)
        ) {
            return;
        }
        this.upsertMessage(message);
        // The owner is looking at it, so the reply never counts as unread.
        if (message.author === AgentConversationAuthor.Agent) {
            this.markRead(message.idConversation);
        }
        if (message.author !== AgentConversationAuthor.User) {
            this.reloadDetail();
        }
    }

    private onTurnStarted(notice: AgentConversationTurnNotice): void {
        if (
            notice.idConversation !== this.idConversation() ||
            this.deletedIds.has(notice.idConversation)
        ) {
            return;
        }
        this.idTaskStreaming.set(notice.idTask);
        this.reloadDetail();
    }

    // The first thinking of a turn also names its task, in case it comes before the turn notice.
    private onThinking(notice: AgentThinkingNotice | null): void {
        const idConversation = this.idConversation();
        if (
            !notice ||
            idConversation === null ||
            notice.idConversation !== idConversation ||
            this.deletedIds.has(idConversation)
        ) {
            return;
        }
        this.idTaskStreaming.set(notice.idTask);
        if (this.isAnswering() || this.isStatusReloading) {
            return;
        }
        this.isStatusReloading = true;
        this.api
            .loadOne$(idConversation)
            .pipe(finalize(() => (this.isStatusReloading = false)))
            .subscribe(detail => {
                if (this.idConversation() === idConversation) {
                    this.applyDetail(detail);
                }
            });
    }

    // Kept once fetched with events.
    private loadThinking(idTask: number): void {
        const idConversation = this.idConversation();
        if (
            idConversation === null ||
            this.thinkingByTask().has(idTask) ||
            this.thinkingStatusByTask().get(idTask) === StoredThinkingStatus.Loading
        ) {
            return;
        }
        this.setThinkingStatus(idTask, StoredThinkingStatus.Loading);
        this.api.loadThinking$(idConversation, idTask).subscribe({
            next: res => {
                if (this.idConversation() !== idConversation) {
                    return;
                }
                if (res.events.length === 0) {
                    this.setThinkingStatus(idTask, StoredThinkingStatus.Empty);
                    return;
                }
                this.thinkingByTask.update(byTask => new Map(byTask).set(idTask, res.events));
                this.setThinkingStatus(idTask, null);
            },
            error: () => {
                if (this.idConversation() === idConversation) {
                    this.setThinkingStatus(idTask, StoredThinkingStatus.Failed);
                }
            }
        });
    }

    private setThinkingStatus(
        idTask: number,
        status: Exclude<StoredThinkingStatus, StoredThinkingStatus.Ready> | null
    ): void {
        this.thinkingStatusByTask.update(byTask => {
            const next = new Map(byTask);
            if (status === null) {
                next.delete(idTask);
            } else {
                next.set(idTask, status);
            }
            return next;
        });
    }

    private reloadDetail(): void {
        const idConversation = this.idConversation();
        if (idConversation === null || this.deletedIds.has(idConversation)) {
            return;
        }
        this.api
            .loadOne$(idConversation)
            .subscribe(detail => this.applyDetailOf(idConversation, detail));
    }

    // A detail that comes back after the owner opened another conversation is dropped.
    private applyDetailOf(idConversation: number, detail: AgentConversationDetail): void {
        if (this.idConversation() === idConversation) {
            this.applyDetail(detail);
        }
    }

    private applyDetail(detail: AgentConversationDetail): void {
        this.detail.set(detail);
        this.linkScope.idProject.set(detail.conversation.idProject);
        if (
            detail.agentStatus.state !== AgentStatusState.Busy ||
            detail.agentStatus.stage !== AgentStage.Chat
        ) {
            this.idTaskStreaming.set(null);
        }
    }

    private markRead(idConversation: number): void {
        this.api.markRead$(idConversation).subscribe(() => {
            this.unreadStore.markRead();
            this.conversations.update(list =>
                list.map(c => (c.idConversation === idConversation ? { ...c, isUnread: false } : c))
            );
        });
    }

    // A stored copy of the owner's message replaces the first pending one with the same text, whether
    // it comes back as the response or as a notice; the second of the two finds it stored already.
    private upsertMessage(message: AgentConversationMessage): void {
        this.messages.update(list => {
            if (list.some(m => m.idMessage === message.idMessage)) {
                return list;
            }
            const index =
                message.author === AgentConversationAuthor.User
                    ? list.findIndex(m => m.idMessage < 0 && m.body === message.body)
                    : -1;
            const kept = index >= 0 ? list.filter((_, i) => i !== index) : list;
            return [...kept, message];
        });
    }

    private scrollToBottom(): void {
        setTimeout(() => {
            const element = this.msgList()?.nativeElement;
            if (element) {
                element.scrollTop = element.scrollHeight;
            }
        }, 0);
    }

    private static isUnread(conversation: AgentConversation): boolean {
        const lastReply = conversation.lastAgentMessageAt;
        if (lastReply === null) {
            return false;
        }
        const readAt = conversation.readAt;
        return readAt === null || new Date(lastReply).getTime() > new Date(readAt).getTime();
    }

    private static toId(value: string | null): number | null {
        const id = Number(value);
        return value !== null && Number.isInteger(id) && id > 0 ? id : null;
    }
}
