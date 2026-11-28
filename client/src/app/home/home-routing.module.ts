import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';
import { SeverityResolver } from '../severity/severity.resolver';
import { StateResolver } from '../state/state.resolver';
import { IssueTypeResolver } from '../issue-type/issue-type.resolver';
import { TagResolver } from '../tag/tag.resolver';
import { ShellLayoutComponent } from './layouts/shell-layout/shell-layout.component';
import { FirstProjectGuard } from '../onboarding/first-project.guard';
import { LastConversationGuard } from '../message/guards/last-conversation.guard';

const routes: Routes = [
    {
        path: '',
        component: ShellLayoutComponent,
        children: [
            {
                path: 'project',
                resolve: {
                    severities: SeverityResolver,
                    state: StateResolver,
                    issueTypes: IssueTypeResolver,
                    tags: TagResolver
                },
                loadChildren: () => import('../project/project.module').then(m => m.ProjectModule)
            },
            {
                path: 'user',
                loadChildren: () => import('../user/user.module').then(m => m.UserModule)
            },
            {
                path: 'message',
                pathMatch: 'full',
                canActivate: [LastConversationGuard.canActivate],
                children: []
            },
            {
                path: 'message',
                loadChildren: () => import('../message/message.module').then(m => m.MessageModule)
            },
            {
                path: 'agent-chat',
                loadChildren: () =>
                    import('../agent-chat/agent-chat.module').then(m => m.AgentChatModule)
            },
            {
                path: 'admin',
                loadChildren: () => import('../admin/admin.module').then(m => m.AdminModule)
            },
            {
                path: '',
                canMatch: [FirstProjectGuard.canMatch],
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
