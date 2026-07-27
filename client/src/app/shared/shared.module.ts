import { NgModule } from '@angular/core';
import { WindowModule } from './window/window.module';
import { AvatarComponent } from './avatar/avatar.component';
import { UserDropdownComponent } from './user-dropdown/user-dropdown.component';
import { CoreModule } from '../core/core.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { SeverityDropdownComponent } from './severity-dropdown/severity-dropdown.component';
import { StateDropdownComponent } from './state-dropdown/state-dropdown.component';
import { TrackerComponent } from './tracker/tracker.component';
import { DurationPipe } from './duration/duration.pipe';
import { RouterModule } from '@angular/router';
import { EmojiPipe } from './emoji/emoji.pipe';
import { SecDurationPipe } from './duration/sec-duration.pipe';
import { StateBadgeComponent } from './state-badge/state-badge.component';
import { StateBadgeSelectorComponent } from './state-badge-selector/state-badge-selector.component';
import { SeverityBadgeComponent } from './severity-badge/severity-badge.component';
import { SeverityBadgeSelectorComponent } from './severity-badge-selector/severity-badge-selector.component';
import { TeammateSelectorComponent } from './teammate-selector/teammate-selector.component';
import { StagedIssueComponent } from './staged-issue/staged-issue.component';
import { MockupCardComponent } from './components/mockup-card/mockup-card.component';
import { MentionChipComponent } from './mention/mention-chip/mention-chip.component';
import { MessageBodyComponent } from './mention/message-body/message-body.component';
import { DiffViewerComponent } from './components/diff-viewer/diff-viewer.component';
import { MarkdownModule } from 'ngx-markdown';
import { UiModule } from '../ui/ui.module';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconX,
    IconCheck,
    IconClock,
    IconCircleFilled,
    IconDeviceImac
} from '@tabler/icons-angular';

@NgModule({
    declarations: [
        AvatarComponent,
        StateBadgeComponent,
        StateBadgeSelectorComponent,
        SeverityBadgeComponent,
        SeverityBadgeSelectorComponent,
        UserDropdownComponent,
        SeverityDropdownComponent,
        StateDropdownComponent,
        TrackerComponent,
        DurationPipe,
        SecDurationPipe,
        EmojiPipe,
        TeammateSelectorComponent,
        StagedIssueComponent,
        MockupCardComponent,
        MentionChipComponent,
        MessageBodyComponent,
        DiffViewerComponent
    ],
    imports: [
        CoreModule,
        RouterModule,
        FormsModule,
        ReactiveFormsModule,
        MarkdownModule.forChild(),
        UiModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({ IconX, IconCheck, IconClock, IconCircleFilled, IconDeviceImac })
    ],
    exports: [
        UiModule,
        WindowModule,
        AvatarComponent,
        StateBadgeComponent,
        StateBadgeSelectorComponent,
        SeverityBadgeComponent,
        SeverityBadgeSelectorComponent,
        UserDropdownComponent,
        SeverityDropdownComponent,
        StateDropdownComponent,
        TrackerComponent,
        DurationPipe,
        SecDurationPipe,
        EmojiPipe,
        TeammateSelectorComponent,
        StagedIssueComponent,
        MockupCardComponent,
        MentionChipComponent,
        MessageBodyComponent,
        DiffViewerComponent
    ]
})
export class SharedModule {}
