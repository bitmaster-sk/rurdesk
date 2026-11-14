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
    IconRestore,
    IconShieldLock,
    IconLock,
    IconPlugConnected,
    IconListCheck,
    IconShield,
    IconShieldCheck,
    IconBook,
    IconCircle,
    IconCircleCheck,
    IconInfoCircle,
    IconArrowUpRight,
    IconArrowDownLeft,
    IconArrowRight,
    IconChevronRight
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
import { AdminSettingsLayoutComponent } from './pages/admin-settings/admin-settings-layout.component';
import { AdminSettingsGeneralComponent } from './pages/admin-settings/general/admin-settings-general.component';
import { AdminSettingsAgentsComponent } from './pages/admin-settings/agents/admin-settings-agents.component';
import { AdminSettingsSsoComponent } from './pages/admin-settings/sso/admin-settings-sso.component';
import { SsoIdpDataComponent } from './components/sso-idp-data/sso-idp-data.component';
import { SsoConnectionFormComponent } from './components/sso-connection-form/sso-connection-form.component';
import { SsoTestResultComponent } from './components/sso-test-result/sso-test-result.component';
import { SsoStatusComponent } from './components/sso-status/sso-status.component';
import { SsoHintComponent } from './components/sso-hint/sso-hint.component';
import { SsoRulesFormComponent } from './components/sso-rules-form/sso-rules-form.component';
import { SsoGroupMappingComponent } from './components/sso-group-mapping/sso-group-mapping.component';
import { SsoEnforcementComponent } from './components/sso-enforcement/sso-enforcement.component';
import { SsoScimComponent } from './components/sso-scim/sso-scim.component';
import { AdminSkillsComponent } from './pages/admin-skills/admin-skills.component';

@NgModule({
    declarations: [
        AdminUsersComponent,
        AdminSettingsLayoutComponent,
        AdminSettingsGeneralComponent,
        AdminSettingsAgentsComponent,
        AdminSettingsSsoComponent,
        SsoIdpDataComponent,
        SsoConnectionFormComponent,
        SsoTestResultComponent,
        SsoStatusComponent,
        SsoHintComponent,
        SsoRulesFormComponent,
        SsoGroupMappingComponent,
        SsoEnforcementComponent,
        SsoScimComponent,
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
            IconRestore,
            IconShieldLock,
            IconLock,
            IconPlugConnected,
            IconListCheck,
            IconShield,
            IconShieldCheck,
            IconBook,
            IconCircle,
            IconCircleCheck,
            IconInfoCircle,
            IconArrowUpRight,
            IconArrowDownLeft,
            IconArrowRight,
            IconChevronRight
        })
    ]
})
export class AdminModule {}
