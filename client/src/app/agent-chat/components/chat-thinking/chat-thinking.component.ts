import {
    ChangeDetectionStrategy,
    Component,
    OnDestroy,
    computed,
    effect,
    inject,
    input,
    output,
    signal,
    untracked
} from '@angular/core';
import { AgentThinkingConverter } from 'src/app/agent/converter/agent-thinking.converter';
import { AgentThinkingEvent } from 'src/app/agent/model/agent-thinking.model';
import { AgentThinkingStore } from 'src/app/agent/store/agent-thinking.store';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { AgentConversationApi } from '../../api/agent-conversation.api.service';
import { LiveThinking } from '../../entity/live-thinking.entity';
import { StoredThinkingStatus } from '../../constants/stored-thinking-status.enum';

/**
 * The agent's thinking in a conversation: live below the last message while a
 * turn runs, or the given events of a finished reply.
 */
@Component({
    selector: 'app-chat-thinking',
    templateUrl: './chat-thinking.component.html',
    styleUrls: ['./chat-thinking.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false,
    providers: [AgentThinkingStore]
})
export class ChatThinkingComponent implements OnDestroy {
    /** The conversation, run and turn the live block follows; a finished reply needs none. */
    public readonly idConversation = input<number | null>(null);
    public readonly idRun = input<number | null>(null);
    /** The running turn when the page knows it before the stream names it. */
    public readonly idTask = input<number | null>(null);
    public readonly isLive = input(false);
    /** The thinking of a finished reply. */
    public readonly events = input<AgentThinkingEvent[]>([]);
    public readonly status = input<StoredThinkingStatus>(StoredThinkingStatus.Ready);

    protected readonly StoredThinkingStatus = StoredThinkingStatus;

    /** What streamed so far for the running turn, so its reply can show it without a fetch. */
    public readonly thinkingChange = output<LiveThinking>();

    private readonly api = inject(AgentConversationApi);
    private readonly store = inject(AgentThinkingStore);

    private idTaskJoined: number | null = null;

    private readonly startedAt = Date.now();
    private readonly now = signal(Date.now());
    private ticker: ReturnType<typeof setInterval> | null = null;

    protected readonly lines = computed(() =>
        this.isLive() ? this.store.lines() : AgentThinkingConverter.toLines(this.events())
    );

    protected readonly hasGap = computed(() => this.isLive() && this.store.hasGap());

    protected readonly isWaiting = computed(
        () => this.isLive() && this.store.events().length === 0
    );

    protected readonly elapsed = computed(() => {
        const seconds = Math.max(0, Math.floor((this.now() - this.startedAt) / 1000));
        return DurationFormatter.durationToString(DurationConverter.secondsToDuration(seconds));
    });

    public constructor() {
        effect(() => {
            if (!this.isLive()) {
                return;
            }
            const idRun = this.idRun();
            untracked(() => this.store.bind(idRun));
            if (this.ticker === null) {
                this.ticker = setInterval(() => this.now.set(Date.now()), 1000);
            }
        });
        effect(() => {
            if (!this.isLive()) {
                return;
            }
            // The stream names the newest turn; before it starts, the page's does.
            const streaming = this.store.idTask();
            const idTask = streaming ?? this.idTask();
            if (idTask === null || idTask === this.idTaskJoined) {
                return;
            }
            // A turn streamed from its first batch has nothing to catch up on.
            const hasMissed = streaming === null || this.store.hasGap();
            this.idTaskJoined = idTask;
            if (hasMissed) {
                untracked(() => this.joinTurn(idTask));
            }
        });
        effect(() => {
            const idTask = this.store.idTask();
            const events = this.store.events();
            const hasGap = this.store.hasGap();
            if (this.isLive() && idTask !== null && events.length > 0) {
                untracked(() => this.thinkingChange.emit({ idTask, events, hasGap }));
            }
        });
    }

    public ngOnDestroy(): void {
        if (this.ticker !== null) {
            clearInterval(this.ticker);
        }
    }

    // The stream only carries what arrives from now on; the stored copy goes in front, and the store
    // drops streamed batches it already holds.
    private joinTurn(idTask: number): void {
        const idConversation = this.idConversation();
        if (idConversation === null) {
            this.idTaskJoined = null;
            return;
        }
        this.api.loadThinking$(idConversation, idTask).subscribe({
            next: res => this.store.applyStored(res.stage, idTask, res.events, res.lastSeq ?? 0),
            error: () => (this.idTaskJoined = null)
        });
    }
}
