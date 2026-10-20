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
    IconLock
} from '@tabler/icons-angular';
import { AgentRunCardComponent } from './components/agent-run-card/agent-run-card.component';
import { PlanActionsComponent } from './components/plan-actions/plan-actions.component';
import { RunRecoveryBannerComponent } from './components/run-recovery-banner/run-recovery-banner.component';
import { RunStatsPanelComponent } from './components/run-stats-panel/run-stats-panel.component';
import { RunWikiContextComponent } from './components/run-wiki-context/run-wiki-context.component';
import { AgentStagePlanEditorComponent } from './components/agent-stage-plan-editor/agent-stage-plan-editor.component';

@NgModule({
    declarations: [
        AgentRunCardComponent,
        PlanActionsComponent,
        RunRecoveryBannerComponent,
        RunStatsPanelComponent,
        RunWikiContextComponent,
        AgentStagePlanEditorComponent
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
            IconLock
        })
    ],
    exports: [
        AgentRunCardComponent,
        PlanActionsComponent,
        RunRecoveryBannerComponent,
        RunStatsPanelComponent,
        AgentStagePlanEditorComponent
    ]
})
export class AgentModule {}
