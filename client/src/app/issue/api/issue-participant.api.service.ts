import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { IssueParticipantModel } from '../model/issue-participant.model';

@Injectable({ providedIn: 'root' })
export class IssueParticipantApi {
    private readonly http = inject(HttpClient);

    private base(idProject: number, idIssuePublic: number): string {
        return `/api/private/project/${idProject}/issue/${idIssuePublic}/participant`;
    }

    public list$(idProject: number, idIssuePublic: number): Observable<IssueParticipantModel[]> {
        return this.http.get<IssueParticipantModel[]>(this.base(idProject, idIssuePublic));
    }

    public add$(idProject: number, idIssuePublic: number, idUser: number): Observable<void> {
        return this.http.post<void>(this.base(idProject, idIssuePublic), { idUser });
    }

    public setNotifications$(
        idProject: number,
        idIssuePublic: number,
        enabled: boolean
    ): Observable<void> {
        return this.http.patch<void>(`${this.base(idProject, idIssuePublic)}/notifications`, {
            enabled
        });
    }
}
