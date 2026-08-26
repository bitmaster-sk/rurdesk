import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';
import { AdminGuard } from './admin.guard';
import { AdminUsersComponent } from './pages/admin-users/admin-users.component';
import { AdminSettingsComponent } from './pages/admin-settings/admin-settings.component';
import { AdminSkillsComponent } from './pages/admin-skills/admin-skills.component';

const routes: Routes = [
    { path: 'users', component: AdminUsersComponent, canActivate: [AdminGuard] },
    { path: 'settings', component: AdminSettingsComponent, canActivate: [AdminGuard] },
    { path: 'skills', component: AdminSkillsComponent, canActivate: [AdminGuard] },
    { path: '', redirectTo: 'users', pathMatch: 'full' }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class AdminRoutingModule {}
