import {
    ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    computed,
    input,
    output,
    inject,
    signal
} from '@angular/core';
import { UiMenuItem } from 'src/app/ui/components/menu/menu-item.model';
import { UiMenuComponent } from 'src/app/ui/components/menu/menu.component';
import { UiConfirmDirective } from 'src/app/ui/directives/confirm.directive';
import { ConversationGrouper, ConversationSection } from '../../entity/conversation-grouper';
import { AgentConversationListItem } from '../../model/agent-conversation.model';

@Component({
    selector: 'app-conversation-list',
    templateUrl: './conversation-list.component.html',
    styleUrls: ['./conversation-list.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ConversationListComponent {
    private readonly cdr = inject(ChangeDetectorRef);

    public readonly conversations = input<AgentConversationListItem[]>([]);
    public readonly isLoading = input(false);
    public readonly idSelected = input<number | null>(null);
    /** The project the owner works in: a new conversation starts in it and its conversations come first. */
    public readonly idProject = input<number | null>(null);
    public readonly projectName = input<string | null>(null);

    public readonly pinChange = output<{ idConversation: number; isPinned: boolean }>();
    public readonly deleteConversation = output<number>();

    protected readonly filter = signal('');
    /** The conversation whose row menu is open, with the confirmation its Delete opens. */
    private readonly menuTarget = signal<{
        conversation: AgentConversationListItem;
        confirm: UiConfirmDirective;
    } | null>(null);

    protected readonly sections = computed<ConversationSection[]>(() => {
        const text = this.filter().trim().toLocaleLowerCase();
        const list = text
            ? this.conversations().filter(c => c.title.toLocaleLowerCase().includes(text))
            : this.conversations();
        const idProject = this.idProject();
        const name = this.projectName();
        return ConversationGrouper.group(
            list,
            idProject !== null && name !== null ? { idProject, name } : null
        );
    });

    protected readonly rowMenu = computed<UiMenuItem[]>(() => {
        const target = this.menuTarget();
        if (!target) {
            return [];
        }
        const { conversation, confirm } = target;
        const isPinned = conversation.pinnedAt !== null;
        return [
            {
                labelKey: isPinned ? 'AGENT_CHAT.UNPIN' : 'AGENT_CHAT.PIN',
                icon: isPinned ? 'pinned' : 'pin',
                command: () =>
                    this.pinChange.emit({
                        idConversation: conversation.idConversation,
                        isPinned: !isPinned
                    })
            },
            { labelKey: 'AGENT_CHAT.DELETE', icon: 'trash', command: () => confirm.open() }
        ];
    });

    protected isMenuOpenFor(
        conversation: AgentConversationListItem,
        menu: UiMenuComponent
    ): boolean {
        return menu.isOpen() && this.menuTarget()?.conversation === conversation;
    }

    protected onFilterInput(event: Event): void {
        if (event.target instanceof HTMLInputElement) {
            this.filter.set(event.target.value);
        }
    }

    /** One menu serves every row: a click on another row's button moves it there. */
    protected onActions(
        event: Event,
        conversation: AgentConversationListItem,
        menu: UiMenuComponent,
        confirm: UiConfirmDirective
    ): void {
        if (this.isMenuOpenFor(conversation, menu)) {
            menu.hide();
            return;
        }
        menu.hide();
        this.menuTarget.set({ conversation, confirm });
        // The menu renders its rows as it opens, so it needs this row's items already.
        this.cdr.detectChanges();
        menu.show(event);
    }
}
