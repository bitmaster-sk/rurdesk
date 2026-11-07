import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import {
    CreateIssueRelationDto,
    ReadIssueRelationDto,
    SetParentDto,
    SetParentRes
} from '../model/issue-relation.model';

@Injectable({ providedIn: 'root' })
export class IssueRelationApi {
    private readonly http = inject(HttpClient);

    public load$(idProject: number, idsIssue?: number[]): Observable<ReadIssueRelationDto[]> {
        let params = new HttpParams();
        if (idsIssue?.length) {
            params = params.set('idsIssue', idsIssue.join(','));
        }
        return this.http.get<ReadIssueRelationDto[]>(`/api/private/project/${idProject}/relation`, {
            params
        });
    }

    // Lazy per-row load: relations of a single issue (both directions).
    public loadForIssue$(
        idProject: number,
        idIssuePublic: number
    ): Observable<ReadIssueRelationDto[]> {
        return this.http.get<ReadIssueRelationDto[]>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/relation`
        );
    }

    public insert$(
        idProject: number,
        idIssuePublic: number,
        dto: CreateIssueRelationDto
    ): Observable<ReadIssueRelationDto[]> {
        return this.http.post<ReadIssueRelationDto[]>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/relation`,
            dto
        );
    }

    public setParent$(
        idProject: number,
        idIssuePublic: number,
        body: SetParentDto
    ): Observable<SetParentRes> {
        return this.http.put<SetParentRes>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/parent`,
            body
        );
    }

    public delete$(idProject: number, idIssuePublic: number, idRelation: number): Observable<void> {
        return this.http.delete<void>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/relation/${idRelation}`
        );
    }
}
