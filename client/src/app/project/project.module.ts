import { NgModule } from '@angular/core';
import { ProjectPage } from './pages/project/project.page';
import { ProjectRoutingModule } from './project-routing.module';
import { CoreModule } from '../core/core.module';
import { ProjectFormComponent } from './components/project-form/project-form.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { SharedModule } from '../shared/shared.module';
import { ProjectFormWindowComponent } from './components/project-form-window/project-form-window.component';
import { ProjectSettingsPage } from './pages/project-settings/project-settings.page';
import { StateModule } from '../state/state.module';
import { SeverityModule } from '../severity/severity.module';
import { BaseChartDirective } from 'ng2-charts';
import { PinModule } from '../pin/pin.module';
import { RouterModule } from '@angular/router';
import { UiModule } from '../ui/ui.module';
import { ProjectBuilderComponent } from './pages/project-builder/project-builder.component';
import { ProjectBuilderStepInputComponent } from './components/project-builder-step-input/project-builder-step-input.component';
import { ProjectBuilderStepStagingComponent } from './components/project-builder-step-staging/project-builder-step-staging.component';
import { ProjectBuilderStepSuccessComponent } from './components/project-builder-step-success/project-builder-step-success.component';
import { StagedIssueTreeNodeComponent } from './components/staged-issue-tree-node/staged-issue-tree-node.component';
import { StagedIssueTreeComponent } from './components/staged-issue-tree/staged-issue-tree.component';
import { ProjectMembersComponent } from './components/project-members/project-members.component';
import { ProjectLayoutComponent } from './pages/project-layout/project-layout.component';
import { ProjectStatsChartComponent } from './components/project-stats-chart/project-stats-chart.component';
import { StatsBarChartComponent } from './components/stats-bar-chart/stats-bar-chart.component';
import { WorkloadBarListComponent } from './components/workload-bar-list/workload-bar-list.component';
import { GitIntegrationListComponent } from './components/git-integration-list/git-integration-list.component';
import { GitIntegrationSettingsComponent } from './components/git-integration-settings/git-integration-settings.component';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconSettings,
    IconFlag,
    IconAlertTriangle,
    IconDeviceFloppy,
    IconCancel,
    IconUsers,
    IconLock,
    IconTrash,
    IconPlus,
    IconGitBranch,
    IconPencil,
    IconRobot,
    IconBolt,
    IconPin,
    IconChartBar,
    IconSparkles,
    IconArrowLeft,
    IconRefresh,
    IconCheck,
    IconX,
    IconList,
    IconCircleCheck,
    IconHistory,
    IconInfoCircle
} from '@tabler/icons-angular';
import { AgentPhaseStateMapComponent } from './components/agent-phase-state-map/agent-phase-state-map.component';
import { AgentModule } from '../agent/agent.module';
import { CommandRegistryService } from '../core/command/command-registry.service';
import { NavigationCommandProvider } from './command/navigation.command-provider';
import { SavedViewCommandProvider } from './command/saved-view.command-provider';
import { PeopleCommandProvider } from './command/people.command-provider';

@NgModule({
    declarations: [
        ProjectPage,
        ProjectFormComponent,
        ProjectFormWindowComponent,
        ProjectSettingsPage,
        ProjectBuilderComponent,
        ProjectBuilderStepInputComponent,
        ProjectBuilderStepStagingComponent,
        ProjectBuilderStepSuccessComponent,
        StagedIssueTreeNodeComponent,
        StagedIssueTreeComponent,
        ProjectMembersComponent,
        ProjectLayoutComponent,
        ProjectStatsChartComponent,
        StatsBarChartComponent,
        WorkloadBarListComponent,
        GitIntegrationListComponent,
        GitIntegrationSettingsComponent,
        AgentPhaseStateMapComponent
    ],
    imports: [
        CoreModule,
        RouterModule,
        ProjectRoutingModule,
        SharedModule,
        FormsModule,
        ReactiveFormsModule,
        StateModule,
        SeverityModule,
        BaseChartDirective,
        PinModule,
        UiModule,
        TablerIconComponent,
        AgentModule
    ],
    providers: [
        provideTablerIcons({
            IconSettings,
            IconFlag,
            IconAlertTriangle,
            IconDeviceFloppy,
            IconCancel,
            IconUsers,
            IconLock,
            IconTrash,
            IconPlus,
            IconGitBranch,
            IconPencil,
            IconRobot,
            IconBolt,
            IconPin,
            IconChartBar,
            IconSparkles,
            IconArrowLeft,
            IconRefresh,
            IconCheck,
            IconX,
            IconList,
            IconCircleCheck,
            IconHistory,
            IconInfoCircle
        })
    ]
})
export class ProjectModule {
    // Lazy modules can't be collected via an injector token — register imperatively on load.
    public constructor(
        registry: CommandRegistryService,
        nav: NavigationCommandProvider,
        people: PeopleCommandProvider,
        savedViews: SavedViewCommandProvider
    ) {
        registry.register(nav);
        registry.register(people);
        registry.register(savedViews);
    }
}
