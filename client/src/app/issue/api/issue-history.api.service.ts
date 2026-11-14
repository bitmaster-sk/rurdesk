import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { IssueHistoryRes } from '../model/issue-history.model';

@Injectable({ providedIn: 'root' })
export class IssueHistoryApi {
    private readonly http = inject(HttpClient);

    public load$(idProject: number, idIssuePublic: number): Observable<IssueHistoryRes> {
        return this.http.get<IssueHistoryRes>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/history`
        );
    }
}
