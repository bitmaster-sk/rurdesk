import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { UiModule } from '../ui/ui.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconUsers,
    IconUsersGroup,
    IconPencil,
    IconGripVertical,
    IconPlus,
    IconTrash,
    IconKey,
    IconCopy,
    IconRefresh,
    IconUserUp,
    IconUserDown,
    IconUserOff,
    IconUserCheck,
    IconSettings,
    IconDeviceFloppy,
    IconAlertTriangle,
    IconSparkles,
    IconRestore
} from '@tabler/icons-angular';
import { AdminRoutingModule } from './admin-routing.module';
import { SharedModule } from '../shared/shared.module';
import { AdminUsersComponent } from './pages/admin-users/admin-users.component';
import { AdminTeamsComponent } from './components/admin-teams/admin-teams.component';
import { TeamDialogComponent } from './components/team-dialog/team-dialog.component';
import { CreateUserDialogComponent } from './components/create-user-dialog/create-user-dialog.component';
import { AgentRunDefaultsFieldsComponent } from './components/agent-run-defaults-fields/agent-run-defaults-fields.component';
import { AgentModule } from '../agent/agent.module';
import { EditUserDialogComponent } from './components/edit-user-dialog/edit-user-dialog.component';
import { AgentKeysDialogComponent } from './components/agent-keys-dialog/agent-keys-dialog.component';
import { AdminSettingsComponent } from './pages/admin-settings/admin-settings.component';
import { AdminSkillsComponent } from './pages/admin-skills/admin-skills.component';

@NgModule({
    declarations: [
        AdminUsersComponent,
        AdminSettingsComponent,
        AdminSkillsComponent,
        AdminTeamsComponent,
        TeamDialogComponent,
        CreateUserDialogComponent,
        EditUserDialogComponent,
        AgentKeysDialogComponent,
        AgentRunDefaultsFieldsComponent
    ],
    imports: [
        CommonModule,
        FormsModule,
        ReactiveFormsModule,
        TranslateModule,
        AdminRoutingModule,
        SharedModule,
        AgentModule,
        UiModule,
        MarkdownModule.forChild(),
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconUsers,
            IconUsersGroup,
            IconPencil,
            IconGripVertical,
            IconPlus,
            IconTrash,
            IconKey,
            IconCopy,
            IconRefresh,
            IconUserUp,
            IconUserDown,
            IconUserOff,
            IconUserCheck,
            IconSettings,
            IconDeviceFloppy,
            IconAlertTriangle,
            IconSparkles,
            IconRestore
        })
    ]
})
export class AdminModule {}
