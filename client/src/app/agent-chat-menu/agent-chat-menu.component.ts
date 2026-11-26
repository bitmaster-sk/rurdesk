import {
    ChangeDetectionStrategy,
    Component,
    computed,
    DestroyRef,
    inject,
    OnInit,
    signal,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { merge } from 'rxjs';
import { AgentConversationApi } from 'src/app/agent-chat/api/agent-conversation.api.service';
import { AgentConversationListItem } from 'src/app/agent-chat/model/agent-conversation.model';
import { AgentConversationUnreadStore } from 'src/app/agent-chat/store/agent-conversation-unread.store';
import { ProjectContextStore } from 'src/app/home/store/project-context.store';
import { MessageFormatter } from 'src/app/message/formatter/message.formatter';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';
import { UiMenuComponent } from 'src/app/ui/components/menu/menu.component';

const MAX_RECENT = 6;

/** Agent chat menu behind the side menu row; the row itself lives in the side menu. */
@Component({
    selector: 'app-agent-chat-menu',
    templateUrl: './agent-chat-menu.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AgentChatMenuComponent implements OnInit {
    private readonly api = inject(AgentConversationApi);
    private readonly unreadStore = inject(AgentConversationUnreadStore);
    private readonly sNotice = inject(NoticeService);
    private readonly router = inject(Router);
    private readonly destroyRef = inject(DestroyRef);
    private readonly projectContext = inject(ProjectContextStore);

    // Not required: the side menu reads isOpen before this component's view query resolves.
    private readonly menu = viewChild<UiMenuComponent>('agentMenu');

    private readonly conversations = signal<AgentConversationListItem[]>([]);

    public readonly isOpen = computed(() => this.menu()?.isOpen() ?? false);

    /** Unread conversations, for the side menu row. */
    public readonly badge = computed<string>(
        () => MessageFormatter.formatUnreadBadgeText(this.unreadStore.count()) ?? ''
    );

    protected readonly items = computed<UiMenuItem[]>(() => {
        const idProject = this.projectContext.idProject();
        const recent = this.toConversationItems(this.conversations(), idProject);
        return [
            ...recent,
            ...(recent.length > 0 ? [{ separator: true }] : []),
            {
                labelKey: 'AGENT_CHAT.NEW',
                icon: 'plus',
                command: () =>
                    void this.router.navigate(['/agent-chat'], {
                        queryParams: idProject == null ? { new: 1 } : { idProject, new: 1 }
                    })
            },
            {
                labelKey: 'AGENT_CHAT.ALL',
                icon: 'messages',
                command: () => void this.router.navigate(['/agent-chat'])
            }
        ];
    });

    public ngOnInit(): void {
        this.unreadStore.load();
        this.loadConversations();
        merge(this.sNotice.agentConversation$, this.unreadStore.changed$)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.loadConversations());
    }

    public toggle(origin: HTMLElement): void {
        this.menu()?.toggle(origin);
    }

    private loadConversations(): void {
        this.api.load$().subscribe(list => this.conversations.set(list));
    }

    /** Pinned conversations first, then current-project ones, each group keeping the server's order. */
    private toConversationItems(
        list: AgentConversationListItem[],
        idProject: number | null
    ): UiMenuItem[] {
        const pinned = list.filter(c => c.pinnedAt !== null);
        const unpinned = list.filter(c => c.pinnedAt === null);
        const mine = unpinned.filter(c => idProject != null && c.idProject === idProject);
        const others = unpinned.filter(c => !mine.includes(c));
        const recent = [...pinned, ...mine, ...others].slice(0, MAX_RECENT);

        const titles = new Map<string, number>();
        recent.forEach(c => {
            const title = AgentChatMenuComponent.titleOf(c);
            titles.set(title, (titles.get(title) ?? 0) + 1);
        });
        return recent.map(c => {
            const title = AgentChatMenuComponent.titleOf(c);
            return {
                label: (titles.get(title) ?? 0) > 1 ? `${title} #${c.idConversation}` : title,
                icon: c.pinnedAt === null ? 'robot' : 'pin',
                badge: c.isUnread ? '•' : undefined,
                badgeSeverity: 'danger',
                routerLink: ['/agent-chat', c.idConversation]
            };
        });
    }

    private static titleOf(conversation: AgentConversationListItem): string {
        return conversation.title.trim() || `#${conversation.idConversation}`;
    }
}
