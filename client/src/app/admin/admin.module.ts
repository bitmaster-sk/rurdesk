import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { UiModule } from '../ui/ui.module';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
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
import { EditUserDialogComponent } from './components/edit-user-dialog/edit-user-dialog.component';
import { BotKeysDialogComponent } from './components/bot-keys-dialog/bot-keys-dialog.component';
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
        BotKeysDialogComponent
    ],
    imports: [
        CommonModule,
        ReactiveFormsModule,
        TranslateModule,
        AdminRoutingModule,
        SharedModule,
        UiModule,
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
            IconSettings,
            IconDeviceFloppy,
            IconAlertTriangle,
            IconSparkles,
            IconRestore
        })
    ]
})
export class AdminModule {}
