import { NgModule } from '@angular/core';
import { ReactiveFormsModule } from '@angular/forms';
import {
    IconAlertTriangle,
    IconArrowLeft,
    IconArrowRight,
    IconCloudCheck,
    IconExternalLink,
    IconPencil,
    IconPlayerStop,
    IconPlus,
    IconRobot,
    IconRocket,
    IconTrash,
    IconX,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { MarkdownModule } from 'ngx-markdown';
import { AgentModule } from '../agent/agent.module';
import { CoreModule } from '../core/core.module';
import { SeverityModule } from '../severity/severity.module';
import { SharedModule } from '../shared/shared.module';
import { UiModule } from '../ui/ui.module';
import { WikiEditorModule } from '../wiki/wiki-editor.module';
import { ReleaseAgentFormComponent } from './components/release-agent-form/release-agent-form.component';
import { ReleaseDeleteDialogComponent } from './components/release-delete-dialog/release-delete-dialog.component';
import { ReleaseIssueListComponent } from './components/release-issue-list/release-issue-list.component';
import { ReleasePublishedComponent } from './components/release-published/release-published.component';
import { ReleaseTaskTableComponent } from './components/release-task-table/release-task-table.component';
import { ReleaseTasksStepComponent } from './components/release-tasks-step/release-tasks-step.component';
import { ReleaseVersionStepComponent } from './components/release-version-step/release-version-step.component';
import { ReleaseListPage } from './pages/release-list/release-list.page';
import { ReleaseNewPage } from './pages/release-new/release-new.page';
import { ReleasePage } from './pages/release/release.page';
import { ReleaseRoutingModule } from './release-routing.module';

@NgModule({
    declarations: [
        ReleaseListPage,
        ReleaseNewPage,
        ReleasePage,
        ReleaseVersionStepComponent,
        ReleaseTasksStepComponent,
        ReleaseIssueListComponent,
        ReleaseAgentFormComponent,
        ReleasePublishedComponent,
        ReleaseTaskTableComponent,
        ReleaseDeleteDialogComponent
    ],
    imports: [
        CoreModule,
        ReactiveFormsModule,
        SharedModule,
        SeverityModule,
        UiModule,
        AgentModule,
        WikiEditorModule,
        MarkdownModule.forChild(),
        ReleaseRoutingModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconAlertTriangle,
            IconArrowLeft,
            IconArrowRight,
            IconCloudCheck,
            IconExternalLink,
            IconPencil,
            IconPlayerStop,
            IconPlus,
            IconRobot,
            IconRocket,
            IconTrash,
            IconX
        })
    ]
})
export class ReleaseModule {}
