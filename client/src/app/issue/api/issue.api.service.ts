import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { IssuesFilter } from '../components/filter/issue-filter.entity';
import { IssueConverter } from '../converter/issue.converter';
import { Issue, CreateIssueReq } from '../model/issue.model';
import { IssuesPage, IssueGroup } from '../model/issues-page.model';

@Injectable({
    providedIn: 'root'
})
export class IssueApi {
    private readonly http = inject(HttpClient);

    // Legacy array form (unwraps the envelope). Used by callers that want the whole set
    // (calendar/gantt scheduled + backlog) by omitting limit.
    public load$(filter: IssuesFilter): Observable<Issue[]> {
        return this.http
            .get<IssuesPage>(`/api/private/project/${filter.idProject}/issue`, {
                params: IssueConverter.toParams(filter)
            })
            .pipe(map(page => page.items.map(i => IssueConverter.toIssue(i))));
    }

    public loadPage$(
        filter: IssuesFilter,
        limit: number,
        cursor: string | null
    ): Observable<IssuesPage> {
        let params = IssueConverter.toParams(filter).set('limit', String(limit));
        if (cursor) {
            params = params.set('cursor', cursor);
        }
        return this.http
            .get<IssuesPage>(`/api/private/project/${filter.idProject}/issue`, { params })
            .pipe(
                map(page => ({ ...page, items: page.items.map(i => IssueConverter.toIssue(i)) }))
            );
    }

    public loadGrouped$(
        filter: IssuesFilter,
        groupBy: string,
        perGroup: number
    ): Observable<{ groups: IssueGroup[] }> {
        const params = IssueConverter.toParams(filter)
            .set('groupBy', groupBy)
            .set('limit', String(perGroup));
        return this.http
            .get<{ groups: IssueGroup[] }>(`/api/private/project/${filter.idProject}/issue`, {
                params
            })
            .pipe(
                map(res => ({
                    groups: res.groups.map(g => ({
                        ...g,
                        items: g.items.map(i => IssueConverter.toIssue(i))
                    }))
                }))
            );
    }

    public loadOne$(idProject: number, idIssuePublic: number): Observable<Issue> {
        return this.http
            .get<Issue>(`/api/private/project/${idProject}/issue/${idIssuePublic}`)
            .pipe(map(issue => IssueConverter.toIssue(issue)));
    }

    public insert$(issue: CreateIssueReq): Observable<Issue> {
        return this.http
            .post<Issue>(`/api/private/project/${issue.idProject}/issue`, issue)
            .pipe(map(iss => IssueConverter.toIssue(iss)));
    }

    public update$(issue: Issue): Observable<Issue> {
        return this.http
            .patch<Issue>(
                `/api/private/project/${issue.idProject}/issue/${issue.idIssuePublic}`,
                issue
            )
            .pipe(map(iss => IssueConverter.toIssue(iss)));
    }

    public delete$(idProject: number, idIssuePublic: number): Observable<void> {
        return this.http.delete<void>(`/api/private/project/${idProject}/issue/${idIssuePublic}`);
    }
}
