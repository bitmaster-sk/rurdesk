import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
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
    IconCircleMinus
} from '@tabler/icons-angular';
import { AgentRunCardComponent } from './components/agent-run-card/agent-run-card.component';
import { PlanActionsComponent } from './components/plan-actions/plan-actions.component';
import { RunRecoveryBannerComponent } from './components/run-recovery-banner/run-recovery-banner.component';
import { RunStatsPanelComponent } from './components/run-stats-panel/run-stats-panel.component';

@NgModule({
    declarations: [
        AgentRunCardComponent,
        PlanActionsComponent,
        RunRecoveryBannerComponent,
        RunStatsPanelComponent
    ],
    imports: [CommonModule, ReactiveFormsModule, TranslateModule, UiModule, TablerIconComponent],
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
            IconCircleMinus
        })
    ],
    exports: [
        AgentRunCardComponent,
        PlanActionsComponent,
        RunRecoveryBannerComponent,
        RunStatsPanelComponent
    ]
})
export class AgentModule {}
