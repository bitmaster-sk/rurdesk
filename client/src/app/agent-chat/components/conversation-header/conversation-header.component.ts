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
import { FormControl } from '@angular/forms';
import { CostFormatter } from 'src/app/agent/entity/cost.formatter';
import { TokenFormatter } from 'src/app/agent/entity/token.formatter';
import { Project } from 'src/app/project/model/project.model';
import { AgentChatState } from '../../constants/agent-chat-state.enum';
import { AgentChatStatus } from '../../entity/agent-chat-status.entity';
import { AgentChatStatusConverter } from '../../entity/agent-chat-status.converter';
import {
    AgentConversationDetail,
    AgentConversationSettings
} from '../../model/agent-conversation.model';

/** Title, agent, agent status, model, tokens and the projects a conversation reads. */
@Component({
    selector: 'app-conversation-header',
    templateUrl: './conversation-header.component.html',
    styleUrls: ['./conversation-header.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ConversationHeaderComponent {
    public readonly detail = input.required<AgentConversationDetail>();
    /** The owner's projects, offered as context. */
    public readonly projects = input<Project[]>([]);
    /** Translation key of the last refused attach. */
    public readonly projectError = input<string | null>(null);

    public readonly attachProject = output<number>();
    public readonly detachProject = output<number>();
    public readonly pinChange = output<boolean>();
    /** The owner picked another model; it cannot change while the agent answers. */
    public readonly settingsChange = output<AgentConversationSettings>();
    public readonly deleteConversation = output<void>();
    /** Leaves the conversation for the list; only offered where the two are not side by side. */
    public readonly back = output<void>();

    /** Below this width the model and projects sit in a panel instead of the header. */
    private static readonly phoneQuery = '(max-width: 40rem)';

    private readonly destroyRef = inject(DestroyRef);

    protected readonly isPhone = signal(false);
    protected readonly isSettingsOpen = signal(false);

    protected readonly addProjectControl = new FormControl<number | null>(null);

    protected readonly isLocked = computed(() => this.detail().conversation.isLocked);

    protected readonly hasMain = computed(() =>
        this.detail().projects.some(project => project.isMain)
    );

    protected readonly isPinned = computed(() => this.detail().conversation.pinnedAt !== null);

    protected readonly title = computed(() => this.detail().conversation.title.trim());

    protected readonly projectOptions = computed(() => {
        const attached = new Set(this.detail().projects.map(project => project.idProject));
        return this.projects().filter(project => !attached.has(project.idProject));
    });

    protected readonly tokens = computed(() =>
        TokenFormatter.format(this.detail().stats?.totalTokensUsed ?? 0)
    );

    protected readonly cost = computed(() => {
        const stats = this.detail().stats;
        return stats ? CostFormatter.format(stats.totalCostUsd, stats.costEstimated) : null;
    });

    protected readonly isCostEstimated = computed(
        () => this.detail().stats?.costEstimated ?? false
    );

    protected readonly status = computed<AgentChatStatus>(() =>
        AgentChatStatusConverter.toStatus(this.detail().agentStatus)
    );

    // A new object for every detail, so a reload after a refused change shows the stored model again.
    protected readonly settings = computed<AgentConversationSettings>(() => {
        return { model: this.detail().conversation.model };
    });

    protected readonly isAnswering = computed(
        () => this.status().state === AgentChatState.Answering
    );

    public constructor() {
        this.watchPhoneWidth();
        this.addProjectControl.valueChanges.pipe(takeUntilDestroyed()).subscribe(idProject => {
            if (idProject === null) {
                return;
            }
            this.attachProject.emit(idProject);
            this.addProjectControl.setValue(null, { emitEvent: false });
        });
    }

    private watchPhoneWidth(): void {
        if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
            return;
        }
        const media = window.matchMedia(ConversationHeaderComponent.phoneQuery);
        const apply = (isPhone: boolean): void => {
            this.isPhone.set(isPhone);
            if (!isPhone) {
                this.isSettingsOpen.set(false);
            }
        };
        apply(media.matches);
        const onChange = (event: MediaQueryListEvent): void => apply(event.matches);
        media.addEventListener('change', onChange);
        this.destroyRef.onDestroy(() => media.removeEventListener('change', onChange));
    }
}
