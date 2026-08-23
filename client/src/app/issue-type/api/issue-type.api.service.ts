import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { IssueType } from '../model/issue-type.model';
import { IssueTypeUsage } from '../model/issue-type-usage.model';

@Injectable({
    providedIn: 'root'
})
export class IssueTypeApi {
    private http = inject(HttpClient);

    public load$(): Observable<IssueType[]> {
        return this.http.get<IssueType[]>(`/api/private/issue-type`);
    }

    public insert$(issueType: IssueType): Observable<IssueType> {
        return this.http.post<IssueType>(`/api/private/issue-type`, issueType);
    }

    public update$(issueType: IssueType): Observable<IssueType> {
        return this.http.patch<IssueType>(
            `/api/private/issue-type/${issueType.idIssueType}`,
            issueType
        );
    }

    public usage$(idProject: number, idIssueType: number): Observable<IssueTypeUsage> {
        return this.http.get<IssueTypeUsage>(
            `/api/private/issue-type/${idIssueType}/project/${idProject}/usage`
        );
    }

    public delete$(
        idProject: number,
        idIssueType: number,
        intent?: { migrateTo: number | null }
    ): Observable<void> {
        let params = new HttpParams();
        if (intent) {
            params = params.set(
                'migrateTo',
                intent.migrateTo === null ? 'null' : String(intent.migrateTo)
            );
        }
        return this.http.delete<void>(
            `/api/private/issue-type/${idIssueType}/project/${idProject}`,
            { params }
        );
    }
}
