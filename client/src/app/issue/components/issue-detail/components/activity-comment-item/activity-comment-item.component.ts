import { DOCUMENT } from '@angular/common';
import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    computed,
    effect,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { MessageEditorComponent } from 'src/app/message/components/message-editor/message-editor.component';
import { Message } from 'src/app/message/model/message.model';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { MessageConverter } from 'src/app/message/converter/message.converter';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { User } from 'src/app/auth/model/user.model';
import { MessageSegmentParser } from 'src/app/shared/mention/message-segment.parser';
import { AnchorDraft, AnchorThread } from 'src/app/issue/entity/anchor-thread.entity';
import { AnchorHighlight } from 'src/app/shared/mention/message-body/anchor-dom';
import { AnchorSelectionReader } from 'src/app/shared/mention/message-body/anchor-selection';

/** Highlight key of a thread that is still being written. */
const DRAFT_THREAD = 0;

/** Where the floating reply button sits, relative to the comment card. */
interface SelectionReply {
    draft: AnchorDraft;
    left: number;
    top: number;
}

@Component({
    selector: 'app-activity-comment-item',
    templateUrl: './activity-comment-item.component.html',
    styleUrls: ['./activity-comment-item.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ActivityCommentItemComponent {
    public readonly message = input.required<Message>();
    public readonly isOwnMessage = input(false);
    public readonly isEditing = input(false);
    public readonly agentRun = input<AgentRun | null>(null);
    public readonly threads = input<AnchorThread[]>([]);
    /** A thread being started on this message, shown as a composer under its range. */
    public readonly draft = input<AnchorDraft | null>(null);
    public readonly expandedThreads = input<ReadonlySet<number>>(new Set());
    public readonly unreadByThread = input<ReadonlyMap<number, number>>(new Map());
    public readonly currentUserId = input<number | null>(null);
    public readonly idThreadMessageEdit = input<number | null>(null);
    public readonly candidates = input<Map<number, User> | User[] | null>(null);
    public readonly mentionCandidates = input<User[]>([]);
    // True only when this message is the most recent plan submitted for the
    // current plan_stage. The feed computes it (it can see all messages) and
    // passes it in so the Approve button shows on exactly one plan comment even
    // after a revision adds a newer plan to the thread.
    public readonly isLatestPlanForStage = input(false);

    public readonly editRequest = output();
    public readonly editSave = output<string>();
    public readonly editCancel = output();
    public readonly approve = output<void>();
    public readonly approveMockup = output<string>();
    public readonly anchorRequest = output<AnchorDraft>();
    public readonly draftSend = output<string>();
    public readonly draftCancel = output<void>();
    public readonly threadToggle = output<number>();
    public readonly threadReply = output<{ thread: AnchorThread; text: string }>();
    public readonly threadEditRequest = output<Message>();
    public readonly threadEditSave = output<{ message: Message; text: string }>();
    public readonly threadEditCancel = output<void>();

    protected readonly MessageKind = MessageKind;

    protected readonly isCreatorDisabled = computed(
        () => this.message().creator.isActive === false
    );

    protected readonly attachmentScope = computed(() =>
        MessageConverter.toAttachmentScope(this.message())
    );

    // Per-kind chrome — tabler icon + i18n title. Empty title falls back to no header.
    protected readonly kindIcon: Record<string, string> = {
        [MessageKind.BrainstormingQuestion]: 'question-mark',
        [MessageKind.BrainstormingComplete]: 'circle-check',
        [MessageKind.Design]: 'ruler',
        [MessageKind.ImplementationPlan]: 'list-check',
        [MessageKind.PullRequestPushed]: 'code',
        [MessageKind.ImplementationDone]: 'flag',
        [MessageKind.ReviewReply]: 'message-reply'
    };

    protected readonly kindTitleKey: Record<string, string> = {
        [MessageKind.BrainstormingQuestion]: 'AGENT.KIND.BRAINSTORMING_QUESTION',
        [MessageKind.BrainstormingComplete]: 'AGENT.KIND.BRAINSTORMING_COMPLETE',
        [MessageKind.Design]: 'AGENT.KIND.DESIGN',
        [MessageKind.ImplementationPlan]: 'AGENT.KIND.IMPLEMENTATION_PLAN',
        [MessageKind.PullRequestPushed]: 'AGENT.KIND.PULL_REQUEST_PUSHED',
        [MessageKind.ImplementationDone]: 'AGENT.KIND.IMPLEMENTATION_DONE',
        [MessageKind.ReviewReply]: 'AGENT.KIND.REVIEW_REPLY'
    };

    protected readonly kindHeaderTitle = computed(
        () => this.kindTitleKey[this.message().messageKind] ?? ''
    );
    protected readonly kindHeaderIcon = computed(
        () => this.kindIcon[this.message().messageKind] ?? ''
    );

    private readonly editorRef = viewChild<MessageEditorComponent>(MessageEditorComponent);

    private readonly focusEffect = effect(() => {
        if (this.isEditing() && this.editorRef()) {
            setTimeout(() => this.editorRef()?.focus(), 0);
        }
    });

    protected readonly isPlanComment = computed(() => {
        const run = this.agentRun();
        if (run?.phase !== AgentPhase.AwaitingApproval) {
            return false;
        }
        const kind = this.message().messageKind;
        return kind === MessageKind.Design || kind === MessageKind.ImplementationPlan;
    });

    protected readonly isClarificationComment = computed(
        () => this.message().messageKind === MessageKind.BrainstormingQuestion
    );

    // A design message can embed one or more ```mockup blocks. When it does, the
    // approval moves onto each mockup card (use & approve) and the single global
    // Approve button is suppressed to avoid an ambiguous "which mockup?" control.
    // Derived from the same parser that renders the cards (not a substring check)
    // so a malformed/unclosed fence can never hide the global approve while
    // rendering zero cards — which would leave the run unapprovable.
    protected readonly hasMockups = computed(() =>
        MessageSegmentParser.parse(this.message().message).some(s => s.type === 'mockup')
    );

    protected readonly showApproveActions = computed(
        () => this.isPlanComment() && this.isLatestPlanForStage() && !this.hasMockups()
    );

    // Mockup cards become approvable in exactly the same window the global Approve
    // button would have shown, but only for a message that actually has mockups.
    protected readonly mockupApprovable = computed(
        () => this.isPlanComment() && this.isLatestPlanForStage() && this.hasMockups()
    );

    // The approved mockup ref persisted on the run drives the selected/rejected
    // badges (survives reload; independent of the awaiting-approval window).
    protected readonly selectedMockupRef = computed(
        () => this.agentRun()?.approvedMockupRef ?? null
    );

    protected readonly liveThreads = computed(() =>
        this.threads().filter(thread => !thread.isOutdated)
    );
    protected readonly outdatedThreads = computed(() =>
        this.threads().filter(thread => thread.isOutdated)
    );

    protected readonly anchorBreaks = computed<number[]>(() => {
        const draft = this.draft();
        const lines = this.liveThreads().map(thread => thread.lineEnd);
        return [...new Set(draft ? [...lines, draft.lineEnd] : lines)];
    });

    protected readonly anchorHighlights = computed<AnchorHighlight[]>(() => {
        const draft = this.draft();
        const highlights: AnchorHighlight[] = this.liveThreads().map(thread => ({
            idThread: thread.idThread,
            lineStart: thread.lineStart,
            lineEnd: thread.lineEnd,
            quote: thread.quote
        }));
        return draft ? [...highlights, { ...draft, idThread: DRAFT_THREAD }] : highlights;
    });

    /** Live threads keyed by the line their range ends on, where the body renders them. */
    protected readonly threadsByEnd = computed(() => {
        const byEnd = new Map<number, AnchorThread[]>();
        for (const thread of this.liveThreads()) {
            byEnd.set(thread.lineEnd, [...(byEnd.get(thread.lineEnd) ?? []), thread]);
        }
        return byEnd;
    });

    protected readonly selectionReply = signal<SelectionReply | null>(null);

    private readonly bodyHost = viewChild<ElementRef<HTMLElement>>('bodyHost');
    private readonly card = viewChild<ElementRef<HTMLElement>>('card');

    public constructor() {
        // A selection can end outside the body or move to another message, so it is read on
        // every mouseup/keyup in the page, not only inside this body.
        const document = inject(DOCUMENT);
        document.addEventListener('mouseup', this.onSelectionEnd);
        document.addEventListener('keyup', this.onSelectionEnd);
        inject(DestroyRef).onDestroy(() => {
            document.removeEventListener('mouseup', this.onSelectionEnd);
            document.removeEventListener('keyup', this.onSelectionEnd);
        });
    }

    private readonly onSelectionEnd = (): void => {
        const host = this.bodyHost()?.nativeElement;
        const anchorNode = window.getSelection()?.anchorNode ?? null;
        if (!this.selectionReply() && !(host && anchorNode && host.contains(anchorNode))) {
            return;
        }
        const card = this.card()?.nativeElement;
        const selection = host
            ? AnchorSelectionReader.read(window.getSelection(), host, 'app-anchor-thread')
            : null;
        if (!selection || !card) {
            this.selectionReply.set(null);
            return;
        }
        const cardRect = card.getBoundingClientRect();
        this.selectionReply.set({
            draft: {
                idParentMessage: this.message().idMessage,
                lineStart: selection.lineStart,
                lineEnd: selection.lineEnd,
                quote: selection.quote
            },
            left: selection.rect.left + selection.rect.width / 2 - cardRect.left,
            top: selection.rect.top - cardRect.top
        });
    };

    protected onReplyToSelection(): void {
        const reply = this.selectionReply();
        if (!reply) {
            return;
        }
        this.selectionReply.set(null);
        window.getSelection()?.removeAllRanges();
        this.anchorRequest.emit(reply.draft);
    }

    protected onAnchorClick(idThread: number): void {
        if (idThread !== DRAFT_THREAD && !this.expandedThreads().has(idThread)) {
            this.threadToggle.emit(idThread);
        }
    }

    protected onEditCancel(): void {
        this.editCancel.emit();
    }
}
