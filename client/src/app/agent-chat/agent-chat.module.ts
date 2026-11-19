import { NgModule } from '@angular/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import {
    IconAdjustmentsHorizontal,
    IconArrowLeft,
    IconBooks,
    IconBrain,
    IconClipboardList,
    IconCode,
    IconPin,
    IconPinned,
    IconPlus,
    IconTrash,
    IconX,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { CoreModule } from '../core/core.module';
import { MessageModule } from '../message/message.module';
import { SharedModule } from '../shared/shared.module';
import { AgentChatRoutingModule } from './agent-chat-routing.module';
import { ChatModelSettingsComponent } from './components/chat-model-settings/chat-model-settings.component';
import { ChatRowComponent } from './components/chat-row/chat-row.component';
import { ChatThinkingComponent } from './components/chat-thinking/chat-thinking.component';
import { ChatThinkingPillComponent } from './components/chat-thinking-pill/chat-thinking-pill.component';
import { ConversationHeaderComponent } from './components/conversation-header/conversation-header.component';
import { ConversationListComponent } from './components/conversation-list/conversation-list.component';
import { NewConversationComponent } from './components/new-conversation/new-conversation.component';
import { TaskDraftCardComponent } from './components/task-draft-card/task-draft-card.component';
import { TaskDraftDialogComponent } from './components/task-draft-dialog/task-draft-dialog.component';
import { AgentChatPage } from './pages/agent-chat/agent-chat.page';

@NgModule({
    declarations: [
        AgentChatPage,
        ConversationListComponent,
        ConversationHeaderComponent,
        NewConversationComponent,
        ChatModelSettingsComponent,
        ChatRowComponent,
        ChatThinkingComponent,
        ChatThinkingPillComponent,
        TaskDraftCardComponent,
        TaskDraftDialogComponent
    ],
    imports: [
        // Before MessageModule, whose own routes would otherwise be tried first.
        AgentChatRoutingModule,
        CoreModule,
        SharedModule,
        MessageModule,
        FormsModule,
        ReactiveFormsModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconAdjustmentsHorizontal,
            IconArrowLeft,
            IconBooks,
            IconBrain,
            IconClipboardList,
            IconCode,
            IconPin,
            IconPinned,
            IconPlus,
            IconTrash,
            IconX
        })
    ]
})
export class AgentChatModule {}
