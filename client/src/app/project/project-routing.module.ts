import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';
import { ProjectSettingsPage } from './pages/project-settings/project-settings.page';
import { ProjectResolver } from './project.resolver';
import { ProjectPage } from './pages/project/project.page';
import { ProjectMemberResolver } from './project-member.resolver';
import { ProjectBuilderComponent } from './pages/project-builder/project-builder.component';
import { ProjectLayoutComponent } from './pages/project-layout/project-layout.component';
import { projectOwnerGuard } from './project-owner.guard';

const routes: Routes = [
    {
        path: ':idProject',
        component: ProjectLayoutComponent,
        resolve: {
            project: ProjectResolver,
            users: ProjectMemberResolver
        },
        children: [
            { path: 'view', component: ProjectPage },
            { path: 'settings', component: ProjectSettingsPage, canActivate: [projectOwnerGuard] },
            { path: 'project-builder', component: ProjectBuilderComponent },
            {
                path: 'issue',
                loadChildren: () => import('../issue/issue.module').then(m => m.IssueModule)
            }
        ]
    }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class ProjectRoutingModule {}
