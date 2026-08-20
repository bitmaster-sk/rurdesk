import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { UiModule } from '../ui/ui.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
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
    IconAlertTriangle
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

@NgModule({
    declarations: [
        AdminUsersComponent,
        AdminSettingsComponent,
        AdminTeamsComponent,
        TeamDialogComponent,
        CreateUserDialogComponent,
        EditUserDialogComponent,
        BotKeysDialogComponent
    ],
    imports: [
        CommonModule,
        ReactiveFormsModule,
        FormsModule,
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
            IconAlertTriangle
        })
    ]
})
export class AdminModule {}
