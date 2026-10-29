import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { ReleaseListPage } from './pages/release-list/release-list.page';
import { ReleaseNewPage } from './pages/release-new/release-new.page';
import { ReleasePage } from './pages/release/release.page';

const routes: Routes = [
    { path: '', component: ReleaseListPage },
    { path: 'new', component: ReleaseNewPage },
    { path: ':idRelease', component: ReleasePage }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class ReleaseRoutingModule {}
