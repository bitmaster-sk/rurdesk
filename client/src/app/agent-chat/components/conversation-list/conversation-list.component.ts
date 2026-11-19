import {
    ChangeDetectionStrategy,
    Component,
    effect,
    input,
    output,
    untracked
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl } from '@angular/forms';
import { CostFormatter } from 'src/app/agent/entity/cost.formatter';
import { TokenFormatter } from 'src/app/agent/entity/token.formatter';
import { AgentChatScope } from '../../constants/agent-chat-scope.enum';
import { AgentConversationListItem } from '../../model/agent-conversation.model';

@Component({
    selector: 'app-conversation-list',
    templateUrl: './conversation-list.component.html',
    styleUrls: ['./conversation-list.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class ConversationListComponent {
    public readonly conversations = input<AgentConversationListItem[]>([]);
    public readonly idSelected = input<number | null>(null);
    /** The project the page was opened for; without one the list has no scope toggle. */
    public readonly idProject = input<number | null>(null);
    public readonly scope = input<AgentChatScope>(AgentChatScope.All);

    public readonly scopeChange = output<AgentChatScope>();
    public readonly pinChange = output<{ idConversation: number; isPinned: boolean }>();
    public readonly deleteConversation = output<number>();

    protected readonly scopeControl = new FormControl<AgentChatScope>(AgentChatScope.All, {
        nonNullable: true
    });
    protected readonly scopeOptions = [
        { labelKey: 'AGENT_CHAT.SCOPE_PROJECT', value: AgentChatScope.Project },
        { labelKey: 'AGENT_CHAT.SCOPE_ALL', value: AgentChatScope.All }
    ];

    protected readonly TokenFormatter = TokenFormatter;
    protected readonly CostFormatter = CostFormatter;

    protected onPin(conversation: AgentConversationListItem): void {
        this.pinChange.emit({
            idConversation: conversation.idConversation,
            isPinned: conversation.pinnedAt === null
        });
    }

    public constructor() {
        effect(() => {
            const scope = this.scope();
            untracked(() => this.scopeControl.setValue(scope, { emitEvent: false }));
        });
        this.scopeControl.valueChanges
            .pipe(takeUntilDestroyed())
            .subscribe(scope => this.scopeChange.emit(scope));
    }
}
