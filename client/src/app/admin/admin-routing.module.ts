import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';
import { AdminGuard } from './admin.guard';
import { AdminUsersComponent } from './pages/admin-users/admin-users.component';
import { AdminSettingsLayoutComponent } from './pages/admin-settings/admin-settings-layout.component';
import { AdminSettingsGeneralComponent } from './pages/admin-settings/general/admin-settings-general.component';
import { AdminSettingsAgentsComponent } from './pages/admin-settings/agents/admin-settings-agents.component';
import { AdminSettingsSsoComponent } from './pages/admin-settings/sso/admin-settings-sso.component';
import { AdminSkillsComponent } from './pages/admin-skills/admin-skills.component';

const routes: Routes = [
    { path: 'users', component: AdminUsersComponent, canActivate: [AdminGuard] },
    {
        path: 'settings',
        component: AdminSettingsLayoutComponent,
        canActivate: [AdminGuard],
        children: [
            { path: 'general', component: AdminSettingsGeneralComponent },
            { path: 'agents', component: AdminSettingsAgentsComponent },
            { path: 'sso', component: AdminSettingsSsoComponent },
            { path: '', redirectTo: 'general', pathMatch: 'full' }
        ]
    },
    { path: 'skills', component: AdminSkillsComponent, canActivate: [AdminGuard] },
    { path: '', redirectTo: 'users', pathMatch: 'full' }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class AdminRoutingModule {}
