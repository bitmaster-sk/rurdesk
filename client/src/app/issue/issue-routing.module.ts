import { NgModule } from '@angular/core';
import { Routes, RouterModule } from '@angular/router';
import { IssueViewMode } from './constants/issue-view-modes.enum';
import { IssueCalendarComponent } from './components/issue-calendar/issue-calendar.component';
import { IssueDetailPage } from './components/issue-detail/issue-detail.page';
import { IssueKanbanComponent } from './components/issue-kanban/issue-kanban.component';
import { IssueTableComponent } from './components/issue-table/issue-table.component';
import { IssuePage } from './pages/issue/issue.page';
import { IssueGanttComponent } from './components/issue-gantt/issue-gantt.component';

const routes: Routes = [
    {
        path: 'view',
        component: IssuePage,
        children: [
            { path: IssueViewMode.TABLE, component: IssueTableComponent },
            { path: IssueViewMode.KANBAN, component: IssueKanbanComponent },
            { path: IssueViewMode.CALENDAR, component: IssueCalendarComponent },
            { path: IssueViewMode.GANTT, component: IssueGanttComponent },
            { path: '**', redirectTo: IssueViewMode.TABLE }
        ]
    },
    { path: ':idIssuePublic', component: IssueDetailPage },
    { path: '', redirectTo: '/404', pathMatch: 'full' }
];

@NgModule({
    imports: [RouterModule.forChild(routes)],
    exports: [RouterModule]
})
export class IssueRoutingModule {}
