import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';
import { SeverityResolver } from '../severity/severity.resolver';
import { StateResolver } from '../state/state.resolver';
import { ProjectLayoutComponent } from './layouts/project-layout/project-layout.component';
import { AppLayoutComponent } from './layouts/app-layout/app-layout.component';
import { firstProjectGuard } from '../onboarding/first-project.guard';
import { projectLayoutGuard } from './project-layout.guard';

const routes: Routes = [
    {
        path: '',
        component: ProjectLayoutComponent,
        canMatch: [projectLayoutGuard],
        resolve: {
            severities: SeverityResolver,
            state: StateResolver
        },
        children: [
            {
                path: 'project',
                loadChildren: () => import('../project/project.module').then(m => m.ProjectModule)
            }
        ]
    },
    {
        path: '',
        component: AppLayoutComponent,
        children: [
            {
                path: 'user',
                loadChildren: () => import('../user/user.module').then(m => m.UserModule)
            },
            {
                path: 'message',
                loadChildren: () => import('../message/message.module').then(m => m.MessageModule)
            },
            {
                path: 'admin',
                loadChildren: () => import('../admin/admin.module').then(m => m.AdminModule)
            },
            {
                path: '',
                canMatch: [firstProjectGuard],
                loadChildren: () =>
                    import('../onboarding/onboarding.module').then(m => m.OnboardingModule)
            },
            {
                path: '',
                loadChildren: () => import('../user/user.module').then(m => m.UserModule)
            },
            { path: '**', redirectTo: '/404' }
        ]
    }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class HomeRoutingModule {}
