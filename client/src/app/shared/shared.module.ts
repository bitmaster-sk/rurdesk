import { NgModule } from '@angular/core';
import { AgentModule } from '../agent/agent.module';
import { WindowModule } from './window/window.module';
import { AvatarComponent } from './avatar/avatar.component';
import { UserDropdownComponent } from './user-dropdown/user-dropdown.component';
import { CoreModule } from '../core/core.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { IssueCustomFieldsComponent } from './issue-custom-fields/issue-custom-fields.component';
import { SeverityDropdownComponent } from './severity-dropdown/severity-dropdown.component';
import { StateDropdownComponent } from './state-dropdown/state-dropdown.component';
import { TrackerComponent } from './tracker/tracker.component';
import { TrackerControlComponent } from './tracker/tracker-control/tracker-control.component';
import { TrackerStartButtonComponent } from './tracker/tracker-start-button/tracker-start-button.component';
import { TrackerSwitchDialogComponent } from './tracker/tracker-switch-dialog/tracker-switch-dialog.component';
import { TrackEditFormComponent } from './tracker/track-edit-form/track-edit-form.component';
import { DurationPipe } from './duration/duration.pipe';
import { RouterModule } from '@angular/router';
import { SecDurationPipe } from './duration/sec-duration.pipe';
import { StateBadgeComponent } from './state-badge/state-badge.component';
import { StateBadgeSelectorComponent } from './state-badge-selector/state-badge-selector.component';
import { SeverityBadgeComponent } from './severity-badge/severity-badge.component';
import { SeverityBadgeSelectorComponent } from './severity-badge-selector/severity-badge-selector.component';
import { IssueTypeBadgeComponent } from './issue-type-badge/issue-type-badge.component';
import { IssueTypeBadgeSelectorComponent } from './issue-type-badge-selector/issue-type-badge-selector.component';
import { IssueTypeDropdownComponent } from './issue-type-dropdown/issue-type-dropdown.component';
import { TeammateSelectorComponent } from './teammate-selector/teammate-selector.component';
import { StagedIssueComponent } from './staged-issue/staged-issue.component';
import { MockupCardComponent } from './components/mockup-card/mockup-card.component';
import { AgentThinkingRowComponent } from './components/agent-thinking-row/agent-thinking-row.component';

import { MessageBodyComponent } from './mention/message-body/message-body.component';
import { MermaidDiagramComponent } from './mermaid/mermaid-diagram.component';
import { PlantUmlDiagramComponent } from './plantuml/plantuml-diagram.component';
import { AttachmentRenderDirective } from './attachment/directives/attachment-render.directive';
import { IssueLinkCardComponent } from '../link/components/issue-link-card/issue-link-card.component';
import { IssueLinkPickerComponent } from '../link/components/issue-link-picker/issue-link-picker.component';
import { IssueLinkRenderDirective } from '../link/directives/issue-link-render.directive';
import { AttachmentLightboxComponent } from './attachment/components/attachment-lightbox/attachment-lightbox.component';
import { DiffViewerComponent } from './components/diff-viewer/diff-viewer.component';
import { TagChipComponent } from './tag/tag-chip/tag-chip.component';
import { IssueTagsComponent } from './tag/issue-tags/issue-tags.component';
import { TagColorPickerComponent } from './tag/tag-color-picker/tag-color-picker.component';
import { TagInputComponent } from './tag/tag-input/tag-input.component';
import { TagDropdownComponent } from './tag/tag-dropdown/tag-dropdown.component';
import { DeleteMigrationDialogComponent } from './components/delete-migration-dialog/delete-migration-dialog.component';
import { AgentDockComponent } from './components/agent-dock/agent-dock.component';
import { MarkdownModule } from 'ngx-markdown';
import { UiModule } from '../ui/ui.module';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconX,
    IconCheck,
    IconClock,
    IconCircleFilled,
    IconDeviceImac,
    IconRobot,
    IconAdjustmentsHorizontal,
    IconInfoCircle,
    IconPlayerPlay,
    IconPlayerPause,
    IconArrowUpRight,
    IconAlertTriangle,
    IconTrash,
    IconChevronDown,
    IconArrowDown,
    IconArrowRight,
    IconPlus,
    IconBrain,
    IconChevronRight,
    IconTerminal2,
    IconPencil,
    IconSearch,
    IconHammer,
    IconLink
} from '@tabler/icons-angular';

@NgModule({
    declarations: [
        AvatarComponent,
        StateBadgeComponent,
        StateBadgeSelectorComponent,
        SeverityBadgeComponent,
        SeverityBadgeSelectorComponent,
        IssueTypeBadgeComponent,
        IssueTypeBadgeSelectorComponent,
        IssueTypeDropdownComponent,
        UserDropdownComponent,
        SeverityDropdownComponent,
        StateDropdownComponent,
        TrackerComponent,
        TrackerControlComponent,
        TrackerStartButtonComponent,
        TrackerSwitchDialogComponent,
        TrackEditFormComponent,
        DurationPipe,
        SecDurationPipe,
        TeammateSelectorComponent,
        StagedIssueComponent,
        MockupCardComponent,
        AgentThinkingRowComponent,
        MessageBodyComponent,
        MermaidDiagramComponent,
        PlantUmlDiagramComponent,
        AttachmentRenderDirective,
        IssueLinkRenderDirective,
        IssueLinkCardComponent,
        IssueLinkPickerComponent,
        AttachmentLightboxComponent,
        DiffViewerComponent,
        DeleteMigrationDialogComponent,
        AgentDockComponent,
        IssueCustomFieldsComponent,
        TagChipComponent,
        IssueTagsComponent,
        TagColorPickerComponent,
        TagInputComponent,
        TagDropdownComponent
    ],
    imports: [
        AgentModule,
        CoreModule,
        RouterModule,
        FormsModule,
        ReactiveFormsModule,
        MarkdownModule.forChild(),
        UiModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconX,
            IconCheck,
            IconClock,
            IconCircleFilled,
            IconDeviceImac,
            IconRobot,
            IconAdjustmentsHorizontal,
            IconInfoCircle,
            IconPlayerPlay,
            IconPlayerPause,
            IconArrowUpRight,
            IconAlertTriangle,
            IconTrash,
            IconChevronDown,
            IconArrowDown,
            IconArrowRight,
            IconPlus,
            IconBrain,
            IconChevronRight,
            IconTerminal2,
            IconPencil,
            IconSearch,
            IconHammer,
            IconLink
        })
    ],
    exports: [
        UiModule,
        WindowModule,
        AvatarComponent,
        StateBadgeComponent,
        StateBadgeSelectorComponent,
        SeverityBadgeComponent,
        SeverityBadgeSelectorComponent,
        IssueTypeBadgeComponent,
        IssueTypeBadgeSelectorComponent,
        IssueTypeDropdownComponent,
        UserDropdownComponent,
        SeverityDropdownComponent,
        StateDropdownComponent,
        TrackerComponent,
        TrackerControlComponent,
        TrackerStartButtonComponent,
        TrackerSwitchDialogComponent,
        TrackEditFormComponent,
        DurationPipe,
        SecDurationPipe,
        TeammateSelectorComponent,
        StagedIssueComponent,
        MockupCardComponent,
        AgentThinkingRowComponent,
        MessageBodyComponent,
        MermaidDiagramComponent,
        PlantUmlDiagramComponent,
        AttachmentRenderDirective,
        IssueLinkRenderDirective,
        IssueLinkPickerComponent,
        AttachmentLightboxComponent,
        DiffViewerComponent,
        DeleteMigrationDialogComponent,
        AgentDockComponent,
        IssueCustomFieldsComponent,
        TagChipComponent,
        IssueTagsComponent,
        TagColorPickerComponent,
        TagInputComponent,
        TagDropdownComponent
    ]
})
export class SharedModule {}
