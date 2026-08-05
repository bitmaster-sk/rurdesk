import { NgModule } from '@angular/core';
import { IssuePage } from './pages/issue/issue.page';
import { CoreModule } from '../core/core.module';
import { CommandRegistryService } from '../core/command/command-registry.service';
import { IssueActionCommandProvider } from './command/issue-action.command-provider';
import { IssueSearchCommandProvider } from './command/issue-search.command-provider';
import { IssueRoutingModule } from './issue-routing.module';
import { IssueDetailPage } from './components/issue-detail/issue-detail.page';
import { IssueInfoComponent } from './components/issue-detail/components/issue-info/issue-info.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { MessageModule } from '../message/message.module';
import { MarkdownModule } from 'ngx-markdown';
import { SharedModule } from '../shared/shared.module';
import { IssueTableComponent } from './components/issue-table/issue-table.component';
import { IssueKanbanComponent } from './components/issue-kanban/issue-kanban.component';
import { IssueCalendarComponent } from './components/issue-calendar/issue-calendar.component';
import { DragDropModule } from '@angular/cdk/drag-drop';
import { FilterComponent } from './components/filter/filter.component';
import { SeverityModule } from '../severity/severity.module';
import { IssueFilterStore } from './components/filter/issue-filter.store';
import { SavedViewApplyService } from './service/saved-view-apply.service';
import { SavedViewMenuComponent } from './components/saved-view-menu/saved-view-menu.component';
import { SavedViewDialogComponent } from './components/saved-view-dialog/saved-view-dialog.component';
import { IssueToolbarService } from './issue-toolbar.service';
import { KanbanTileComponent } from './components/issue-kanban/components/issue-kanban-tile/issue-kanban-tile.component';
import { TrackerModule } from '../tracker/tracker.module';
import { FullCalendarModule } from '@fullcalendar/angular';
import { IssueTableDropZoneComponent } from './components/issue-table/components/issue-table-drop-zone/issue-table-drop-zone.component';
import { SplitDialogComponent } from './components/split-dialog/split-dialog.component';
import { SplitInputStepComponent } from './components/split-dialog/split-input-step/split-input-step.component';
import { SplitReviewStepComponent } from './components/split-dialog/split-review-step/split-review-step.component';
import { SplitDoneStepComponent } from './components/split-dialog/split-done-step/split-done-step.component';
import { IssueQuickActionsComponent } from './components/issue-quick-actions/issue-quick-actions.component';
import { IssueKanbanSwimlaneComponent } from './components/issue-kanban/components/issue-kanban-swimlane/issue-kanban-swimlane.component';
import { SprintTabStripComponent } from './components/sprint-tab-strip/sprint-tab-strip.component';
import { SprintDialogComponent } from './components/sprint-dialog/sprint-dialog.component';
import { IssueKanbanColumnsComponent } from './components/issue-kanban/components/issue-kanban-columns/issue-kanban-columns.component';
import { QualityBadgeComponent } from './components/quality-badge/quality-badge.component';
import { QualitySuggestionComponent } from './components/quality-suggestion/quality-suggestion.component';
import { QualityPanelComponent } from './components/quality-panel/quality-panel.component';
import { IssueGanttComponent } from './components/issue-gantt/issue-gantt.component';
import { IssueActivityFeedComponent } from './components/issue-detail/components/issue-activity-feed/issue-activity-feed.component';
import { ActivityCommentItemComponent } from './components/issue-detail/components/activity-comment-item/activity-comment-item.component';
import { ActivityTimeItemComponent } from './components/issue-detail/components/activity-time-item/activity-time-item.component';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconFilter,
    IconArrowsSort,
    IconSortAscending,
    IconSortDescending,
    IconAdjustmentsHorizontal,
    IconGripVertical,
    IconLink,
    IconX,
    IconSettings,
    IconDeviceFloppy,
    IconMessage,
    IconClock,
    IconPencil,
    IconArrowLeft,
    IconArrowRight,
    IconSun,
    IconCalendarX,
    IconCalendar,
    IconGitBranch,
    IconExternalLink,
    IconCopy,
    IconTrash,
    IconLayoutColumns,
    IconMap,
    IconRoute,
    IconPlus,
    IconHelpCircle,
    IconChevronDown,
    IconChevronRight,
    IconGauge,
    IconQuestionMark,
    IconCircleCheck,
    IconListCheck,
    IconRuler,
    IconFlag,
    IconCode,
    IconUsers,
    IconBookmark,
    IconBookmarkPlus,
    IconLock,
    IconSearch,
    IconBell,
    IconBellOff,
    IconRepeat
} from '@tabler/icons-angular';
import { GanttTimelineHeaderComponent } from './components/issue-gantt/components/gantt-timeline-header/gantt-timeline-header';
import { GanttTaskBarComponent } from './components/issue-gantt/components/gantt-task-bar/gantt-task-bar';
import { GanttTimelineBodyComponent } from './components/issue-gantt/components/gantt-timeline-body/gantt-timeline-body';
import { GanttWbsPanelComponent } from './components/issue-gantt/components/gantt-wbs-panel/gantt-wbs-panel';
import { GanttArrowLayerComponent } from './components/issue-gantt/components/gantt-arrow-layer/gantt-arrow-layer.component';
import { GanttConnectionHandleComponent } from './components/issue-gantt/components/gantt-connection-handle/gantt-connection-handle.component';
import { GanttMinimapComponent } from './components/issue-gantt/components/gantt-minimap/gantt-minimap.component';
import { MrStatusPillComponent } from './components/issue-detail/components/mr-status-pill/mr-status-pill.component';
import { MrLinkPickerComponent } from './components/issue-detail/components/mr-link-picker/mr-link-picker.component';
import { AnchorReplyComponent } from './components/issue-detail/components/anchor-reply/anchor-reply.component';
import { IssueParticipantsComponent } from './components/issue-detail/components/issue-participants/issue-participants.component';
import { UiModule } from '../ui/ui.module';
import { AgentModule } from '../agent/agent.module';

@NgModule({
    declarations: [
        IssuePage,
        IssueDetailPage,
        IssueInfoComponent,
        IssueTableComponent,
        IssueKanbanComponent,
        IssueCalendarComponent,
        KanbanTileComponent,
        FilterComponent,
        IssueTableDropZoneComponent,
        SplitDialogComponent,
        SplitInputStepComponent,
        SplitReviewStepComponent,
        SplitDoneStepComponent,
        IssueQuickActionsComponent,
        IssueKanbanSwimlaneComponent,
        SprintTabStripComponent,
        SprintDialogComponent,
        IssueKanbanColumnsComponent,
        QualityBadgeComponent,
        QualitySuggestionComponent,
        QualityPanelComponent,
        IssueGanttComponent,
        IssueActivityFeedComponent,
        ActivityCommentItemComponent,
        ActivityTimeItemComponent,
        GanttTimelineHeaderComponent,
        GanttTaskBarComponent,
        GanttTimelineBodyComponent,
        GanttWbsPanelComponent,
        GanttArrowLayerComponent,
        GanttConnectionHandleComponent,
        GanttMinimapComponent,
        MrStatusPillComponent,
        MrLinkPickerComponent,
        AnchorReplyComponent,
        IssueParticipantsComponent,
        SavedViewMenuComponent,
        SavedViewDialogComponent
    ],
    imports: [
        CoreModule,
        SharedModule,
        IssueRoutingModule,
        FormsModule,
        ReactiveFormsModule,
        MessageModule,
        MarkdownModule.forChild(),
        DragDropModule,
        TrackerModule,
        FullCalendarModule,
        TablerIconComponent,
        SeverityModule,
        UiModule,
        AgentModule
    ],
    providers: [
        IssueFilterStore,
        IssueToolbarService,
        SavedViewApplyService,
        provideTablerIcons({
            IconRepeat,
            IconFilter,
            IconArrowsSort,
            IconSortAscending,
            IconSortDescending,
            IconAdjustmentsHorizontal,
            IconGripVertical,
            IconLink,
            IconX,
            IconSettings,
            IconDeviceFloppy,
            IconMessage,
            IconClock,
            IconPencil,
            IconArrowLeft,
            IconArrowRight,
            IconSun,
            IconCalendarX,
            IconCalendar,
            IconGitBranch,
            IconExternalLink,
            IconCopy,
            IconTrash,
            IconLayoutColumns,
            IconMap,
            IconRoute,
            IconPlus,
            IconHelpCircle,
            IconChevronDown,
            IconChevronRight,
            IconGauge,
            IconQuestionMark,
            IconCircleCheck,
            IconListCheck,
            IconRuler,
            IconFlag,
            IconCode,
            IconUsers,
            IconBookmark,
            IconBookmarkPlus,
            IconLock,
            IconSearch,
            IconBell,
            IconBellOff
        })
    ]
})
export class IssueModule {
    // Lazy modules can't be collected via an injector token — register imperatively on load.
    constructor(
        registry: CommandRegistryService,
        actions: IssueActionCommandProvider,
        search: IssueSearchCommandProvider
    ) {
        registry.register(actions);
        registry.register(search);
    }
}
