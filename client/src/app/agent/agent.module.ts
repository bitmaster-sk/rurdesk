import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule } from '@angular/router';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../ui/ui.module';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconRobot,
    IconChevronRight,
    IconGitPullRequest,
    IconX,
    IconPlayerPlay,
    IconRefresh,
    IconCircle,
    IconLoader2,
    IconCircleCheck,
    IconClock,
    IconAlertCircle,
    IconCircleMinus,
    IconSparkles,
    IconFileText,
    IconSearch,
    IconFileOff,
    IconChevronDown,
    IconLock,
    IconAdjustmentsHorizontal,
    IconBook2
} from '@tabler/icons-angular';
import { AgentRunCardComponent } from './components/agent-run-card/agent-run-card.component';
import { PlanActionsComponent } from './components/plan-actions/plan-actions.component';
import { RunRecoveryBannerComponent } from './components/run-recovery-banner/run-recovery-banner.component';
import { RunWikiStageComponent } from './components/run-wiki-stage/run-wiki-stage.component';
import { AgentRunSummaryComponent } from './components/agent-run-summary/agent-run-summary.component';
import { AgentStageCardComponent } from './components/agent-stage-card/agent-stage-card.component';
import { AgentStagePlanEditorComponent } from './components/agent-stage-plan-editor/agent-stage-plan-editor.component';
import { AgentStagePlanFormComponent } from './components/agent-stage-plan-form/agent-stage-plan-form.component';

@NgModule({
    declarations: [
        AgentRunCardComponent,
        PlanActionsComponent,
        RunRecoveryBannerComponent,
        RunWikiStageComponent,
        AgentRunSummaryComponent,
        AgentStageCardComponent,
        AgentStagePlanEditorComponent,
        AgentStagePlanFormComponent
    ],
    imports: [
        CommonModule,
        RouterModule,
        FormsModule,
        ReactiveFormsModule,
        TranslateModule,
        UiModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconRobot,
            IconChevronRight,
            IconGitPullRequest,
            IconX,
            IconPlayerPlay,
            IconRefresh,
            IconCircle,
            IconLoader2,
            IconCircleCheck,
            IconClock,
            IconAlertCircle,
            IconCircleMinus,
            IconSparkles,
            IconFileText,
            IconSearch,
            IconFileOff,
            IconChevronDown,
            IconLock,
            IconAdjustmentsHorizontal,
            IconBook2
        })
    ],
    exports: [
        AgentRunCardComponent,
        PlanActionsComponent,
        RunRecoveryBannerComponent,
        AgentStagePlanEditorComponent,
        AgentStagePlanFormComponent
    ]
})
export class AgentModule {}
