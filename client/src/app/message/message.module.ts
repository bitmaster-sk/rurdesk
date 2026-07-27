import { NgModule } from '@angular/core';

import { MessagePage } from './pages/message/message.page';
import { CoreModule } from '../core/core.module';
import { MessageRoutingModule } from './message-routing.module';
import { MessageEditorComponent } from './components/message-editor/message-editor.component';
import { MessageViewComponent } from './components/message-view/message-view.component';
import { MarkdownModule } from 'ngx-markdown';
import { SharedModule } from '../shared/shared.module';
import { NgClickOutsideDirective } from 'ng-click-outside2';
import { UiModule } from '../ui/ui.module';
import { TranslateModule } from '@ngx-translate/core';
import { MessageMenuComponent } from './components/message-menu/message-menu.component';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconMessage,
    IconMessages,
    IconUsers,
    IconUser,
    IconBold,
    IconItalic,
    IconStrikethrough,
    IconList,
    IconListNumbers,
    IconCode,
    IconX,
    IconSend,
    IconDeviceFloppy,
    IconPencil
} from '@tabler/icons-angular';

@NgModule({
    declarations: [MessagePage, MessageEditorComponent, MessageViewComponent, MessageMenuComponent],
    imports: [
        CoreModule,
        SharedModule,
        MessageRoutingModule,
        MarkdownModule.forChild(),
        NgClickOutsideDirective,
        UiModule,
        TranslateModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconMessage,
            IconMessages,
            IconUsers,
            IconUser,
            IconBold,
            IconItalic,
            IconStrikethrough,
            IconList,
            IconListNumbers,
            IconCode,
            IconX,
            IconSend,
            IconDeviceFloppy,
            IconPencil
        })
    ],
    exports: [MessageEditorComponent, MessageViewComponent, MessageMenuComponent]
})
export class MessageModule {}
