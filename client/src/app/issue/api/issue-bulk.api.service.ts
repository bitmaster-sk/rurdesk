import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';
import { Issue } from '../model/issue.model';
import { BulkEditIssueEntry, BulkEditIssues } from '../model/bulk-edit-issues.model';

@Injectable({ providedIn: 'root' })
export class IssueBulkApi {
    private readonly http = inject(HttpClient);

    public update$(idProject: number, entries: BulkEditIssueEntry[]): Observable<Issue[]> {
        const body: BulkEditIssues = { issues: entries };
        return this.http.patch<Issue[]>(`/api/private/project/${idProject}/issue/batch`, body).pipe(
            map(issues =>
                issues.map(issue => ({
                    ...issue,
                    createAt: issue.createAt ? new Date(issue.createAt) : undefined,
                    updateAt: issue.updateAt ? new Date(issue.updateAt) : undefined,
                    scheduledAt: issue.scheduledAt ? new Date(issue.scheduledAt) : null
                }))
            )
        );
    }
}
