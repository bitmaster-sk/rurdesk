import {
    ChangeDetectionStrategy,
    Component,
    OnDestroy,
    OnInit,
    computed,
    inject
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Router, isActive } from '@angular/router';
import { AgentConversationUnreadStore } from 'src/app/agent-chat/store/agent-conversation-unread.store';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import { HotkeyService } from 'src/app/core/command/hotkey.service';
import { MessageFormatter } from 'src/app/message/formatter/message.formatter';
import { Message } from 'src/app/message/model/message.model';
import { MessageUnreadStore } from 'src/app/message/store/message-unread.store';
import { TrackerControlVariant } from 'src/app/shared/tracker/constants/tracker-control-variant.enum';
import { ProjectContextStore } from '../../store/project-context.store';
import { SideMenuStore } from '../../store/side-menu.store';

@Component({
    selector: 'app-side-menu',
    templateUrl: './side-menu.component.html',
    styleUrls: ['./side-menu.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SideMenuComponent implements OnInit, OnDestroy {
    private readonly router = inject(Router);
    private readonly projectContext = inject(ProjectContextStore);
    private readonly palette = inject(CommandPaletteService);
    private readonly hotkeys = inject(HotkeyService);
    private readonly sideMenuStore = inject(SideMenuStore);
    private readonly messageUnread = inject(MessageUnreadStore);
    private readonly agentChatUnread = inject(AgentConversationUnreadStore);

    protected readonly TrackerControlVariant = TrackerControlVariant;

    protected readonly idProject = this.projectContext.idProject;
    protected readonly canUpdateProject = this.projectContext.canUpdateProject;

    protected readonly isOnMessages = isActive('/message', this.router);
    protected readonly isOnAgentChat = isActive('/agent-chat', this.router);

    protected readonly isCollapsed = this.sideMenuStore.isCollapsed;

    private readonly unreadMessages = toSignal(this.messageUnread.unread$, {
        initialValue: new Map<string, Message[]>()
    });

    /** Unread messages over all chats. */
    protected readonly messagesBadge = computed(() => {
        let count = 0;
        this.unreadMessages().forEach(messages => (count += messages.length));
        return MessageFormatter.formatUnreadBadgeText(count) ?? '';
    });

    /** Conversations with an agent reply not read yet. */
    protected readonly agentChatBadge = computed(
        () => MessageFormatter.formatUnreadBadgeText(this.agentChatUnread.count()) ?? ''
    );

    public ngOnInit(): void {
        this.messageUnread.load();
        this.agentChatUnread.load();
        this.hotkeys.registerRailToggle({ toggle: () => this.sideMenuStore.toggle() });
    }

    public ngOnDestroy(): void {
        this.hotkeys.registerRailToggle(null);
    }

    protected onSearch(): void {
        this.palette.open();
    }

    protected onToggle(): void {
        this.sideMenuStore.toggle();
    }
}
