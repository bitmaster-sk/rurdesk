import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { IssuesFilter } from './components/filter/issue-filter.entity';
import { Issue, IssueDraft } from './model/issue.model';
import { IssuesPage, IssueGroup } from './model/issues-page.model';

@Injectable({
    providedIn: 'root'
})
export class IssueService {
    private readonly http = inject(HttpClient);

    private toParams(filter: IssuesFilter): HttpParams {
        let params = new HttpParams();
        if (filter.orderColumn) {
            params = params.set('orderColumn', filter.orderColumn);
        }
        if (filter.orderDirection) {
            params = params.set('orderDirection', filter.orderDirection);
        }
        if (filter.idsSeverity?.length) {
            params = params.set('idsSeverity', filter.idsSeverity.join(','));
        }
        if (filter.idsState?.length) {
            params = params.set('idsState', filter.idsState.join(','));
        }
        if (filter.idsAssignedTo?.length) {
            params = params.set('idsAssignedTo', filter.idsAssignedTo.join(','));
        }
        if (filter.idsIssuePublic?.length) {
            params = params.set('idsIssuePublic', filter.idsIssuePublic.join(','));
        }
        // A window wins over the absolute pair — sending both would make the URL lie.
        if (filter.createAtWithin) {
            params = params.set('createAtWithin', filter.createAtWithin);
        } else {
            if (filter.createAtFrom) {
                params = params.set('createAtFrom', filter.createAtFrom.toISOString());
            }
            if (filter.createAtTo) {
                params = params.set('createAtTo', filter.createAtTo.toISOString());
            }
        }
        if (filter.updateAtWithin) {
            params = params.set('updateAtWithin', filter.updateAtWithin);
        } else {
            if (filter.updateAtFrom) {
                params = params.set('updateAtFrom', filter.updateAtFrom.toISOString());
            }
            if (filter.updateAtTo) {
                params = params.set('updateAtTo', filter.updateAtTo.toISOString());
            }
        }
        if (filter.scheduledAtFrom) {
            params = params.set('scheduledAtFrom', filter.scheduledAtFrom.toISOString());
        }
        if (filter.scheduledAtTo) {
            params = params.set('scheduledAtTo', filter.scheduledAtTo.toISOString());
        }
        if (filter.scheduledAtUnset) {
            params = params.set('scheduledAtUnset', 'true');
        }
        if (filter.assignedToNull) {
            params = params.set('assignedToNull', 'true');
        }
        if (filter.title) {
            params = params.set('title', filter.title);
        }
        if (filter.sprintUnset) {
            params = params.set('sprintUnset', 'true');
        } else if (filter.idSprint != null) {
            params = params.set('idSprint', String(filter.idSprint));
        }
        params = params.set('stateUnset', filter.stateUnset ? 'true' : 'false');
        params = params.set('severityUnset', filter.severityUnset ? 'true' : 'false');
        params = params.set('assignedToUnset', filter.assignedToUnset ? 'true' : 'false');
        return params;
    }

    // Legacy array form (unwraps the envelope). Used by callers that want the whole set
    // (calendar/gantt scheduled + backlog) by omitting limit.
    public loadIssues(filter: IssuesFilter): Observable<Issue[]> {
        return this.http
            .get<IssuesPage>(`/api/private/project/${filter.idProject}/issue`, {
                params: this.toParams(filter)
            })
            .pipe(map(page => page.items.map(i => this.toIssue(i))));
    }

    public loadIssuesPage$(
        filter: IssuesFilter,
        limit: number,
        cursor: string | null
    ): Observable<IssuesPage> {
        let params = this.toParams(filter).set('limit', String(limit));
        if (cursor) {
            params = params.set('cursor', cursor);
        }
        return this.http
            .get<IssuesPage>(`/api/private/project/${filter.idProject}/issue`, { params })
            .pipe(map(page => ({ ...page, items: page.items.map(i => this.toIssue(i)) })));
    }

    public loadIssuesGrouped$(
        filter: IssuesFilter,
        groupBy: string,
        perGroup: number
    ): Observable<{ groups: IssueGroup[] }> {
        const params = this.toParams(filter).set('groupBy', groupBy).set('limit', String(perGroup));
        return this.http
            .get<{ groups: IssueGroup[] }>(`/api/private/project/${filter.idProject}/issue`, {
                params
            })
            .pipe(
                map(res => ({
                    groups: res.groups.map(g => ({
                        ...g,
                        items: g.items.map(i => this.toIssue(i))
                    }))
                }))
            );
    }

    public loadIssue(idProject: number, idIssuePublic: number): Observable<Issue> {
        return this.http
            .get<Issue>(`/api/private/project/${idProject}/issue/${idIssuePublic}`)
            .pipe(map(issue => this.toIssue(issue)));
    }

    public insertIssue(issue: IssueDraft): Observable<Issue> {
        return this.http
            .post<Issue>(`/api/private/project/${issue.idProject}/issue`, issue)
            .pipe(map(iss => this.toIssue(iss)));
    }

    public updateIssue(issue: Issue): Observable<Issue> {
        return this.http
            .patch<Issue>(
                `/api/private/project/${issue.idProject}/issue/${issue.idIssuePublic}`,
                issue
            )
            .pipe(map(iss => this.toIssue(iss)));
    }

    public deleteIssue(idProject: number, idIssuePublic: number): Observable<void> {
        return this.http.delete<void>(`/api/private/project/${idProject}/issue/${idIssuePublic}`);
    }

    // Public so callers receiving a raw issue over the websocket (NoticeService
    // issue$ payload) can normalize its date fields the same way HTTP responses
    // are normalized.
    public toIssue(issue: Issue): Issue {
        issue.createAt = issue.createAt ? new Date(issue.createAt) : undefined;
        issue.updateAt = issue.updateAt ? new Date(issue.updateAt) : undefined;
        issue.scheduledAt = issue.scheduledAt ? new Date(issue.scheduledAt) : null;
        return issue;
    }
}
