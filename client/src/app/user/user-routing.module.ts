import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';
import { UserSettingsPage } from './pages/user-settings/user-settings.page';
import { UserPage } from './pages/user/user.page';

const routes: Routes = [
    { path: 'settings', component: UserSettingsPage },
    { path: '', component: UserPage }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class UserRoutingModule {}
