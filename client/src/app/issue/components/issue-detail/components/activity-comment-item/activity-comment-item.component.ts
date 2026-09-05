import {
    ChangeDetectionStrategy,
    Component,
    computed,
    effect,
    input,
    output,
    viewChild
} from '@angular/core';
import { MessageEditorComponent } from 'src/app/message/components/message-editor/message-editor.component';
import { Message } from 'src/app/message/model/message.model';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { AgentRun } from 'src/app/agent/model/agent-run.model';
import { AgentPhase } from 'src/app/agent/model/agent-phase.enum';
import { User } from 'src/app/auth/model/user.model';
import { extractMessageSegments } from 'src/app/shared/mention/extract-message-segments';

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
    public readonly anchoredChildren = input<Message[]>([]);
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
    public readonly addAnchor = output<{ lineStart: number; lineEnd: number }>();

    protected readonly MessageKind = MessageKind;

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
        extractMessageSegments(this.message().message).some(s => s.type === 'mockup')
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

    protected readonly lines = computed(() => this.message().message.split('\n'));

    protected readonly childrenByLine = computed(() => {
        const result = new Map<number, Message[]>();
        for (const child of this.anchoredChildren()) {
            if (child.anchor) {
                const key = child.anchor.anchorLineStart;
                const existing = result.get(key);
                if (existing) {
                    existing.push(child);
                } else {
                    result.set(key, [child]);
                }
            }
        }
        return result;
    });

    protected readonly showLineMode = computed(
        () => this.anchoredChildren().length > 0 || this.message().anchor != null
    );

    protected isOutdated(replies: Message[]): boolean {
        return replies.some(r => r.anchor?.isOutdated ?? false);
    }

    protected onAddAnchor(lineNumber: number): void {
        this.addAnchor.emit({ lineStart: lineNumber, lineEnd: lineNumber });
    }

    protected onEditCancel(): void {
        this.editCancel.emit();
    }
}
