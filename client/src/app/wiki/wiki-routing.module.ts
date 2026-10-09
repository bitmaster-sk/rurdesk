import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { WikiEditPage } from './pages/wiki-edit/wiki-edit.page';
import { WikiHistoryPage } from './pages/wiki-history/wiki-history.page';
import { WikiHomePage } from './pages/wiki-home/wiki-home.page';
import { WikiPagePage } from './pages/wiki-page/wiki-page.page';
import { WikiProposalsPage } from './pages/wiki-proposals/wiki-proposals.page';
import { WikiShellPage } from './pages/wiki-shell/wiki-shell.page';
import { WikiTrashPage } from './pages/wiki-trash/wiki-trash.page';
import { WikiEditLeaveGuard } from './wiki-edit-leave.guard';

const routes: Routes = [
    {
        path: '',
        component: WikiShellPage,
        children: [
            { path: '', component: WikiHomePage },
            {
                path: 'new',
                component: WikiEditPage,
                data: { isCreate: true },
                canDeactivate: [WikiEditLeaveGuard.canDeactivate]
            },
            { path: 'trash', component: WikiTrashPage },
            { path: 'proposals', component: WikiProposalsPage },
            { path: 'proposals/:idProposal', component: WikiProposalsPage },
            { path: ':space/:slug', component: WikiPagePage },
            {
                path: ':space/:slug/edit',
                component: WikiEditPage,
                canDeactivate: [WikiEditLeaveGuard.canDeactivate]
            },
            { path: ':space/:slug/history', component: WikiHistoryPage }
        ]
    }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class WikiRoutingModule {}
