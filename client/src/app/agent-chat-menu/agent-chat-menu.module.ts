import { NgModule } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import {
    IconMessages,
    IconPin,
    IconPlus,
    IconRobot,
    provideTablerIcons,
    TablerIconComponent
} from '@tabler/icons-angular';
import { CoreModule } from '../core/core.module';
import { UiModule } from '../ui/ui.module';
import { AgentChatMenuComponent } from './agent-chat-menu.component';

/** Top-menu entry to the agent chat. Non-routing: the conversation pages live in their own module. */
@NgModule({
    declarations: [AgentChatMenuComponent],
    imports: [CoreModule, UiModule, TranslateModule, TablerIconComponent],
    providers: [provideTablerIcons({ IconRobot, IconPlus, IconMessages, IconPin })],
    exports: [AgentChatMenuComponent]
})
export class AgentChatMenuModule {}
