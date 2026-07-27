import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { Sprint, SprintStats } from '../model/sprint.model';

@Injectable({ providedIn: 'root' })
export class SprintApi {
    private readonly http = inject(HttpClient);

    public loadByProject$(idProject: number): Observable<Sprint[]> {
        return this.http.get<Sprint[]>(`/api/private/project/${idProject}/sprint`);
    }

    public create$(idProject: number, body: Partial<Sprint>): Observable<Sprint> {
        return this.http.post<Sprint>(`/api/private/project/${idProject}/sprint`, body);
    }

    public edit$(idSprint: number, body: Partial<Sprint>): Observable<Sprint> {
        return this.http.patch<Sprint>(`/api/private/sprint/${idSprint}`, body);
    }

    public delete$(idSprint: number): Observable<void> {
        return this.http.delete<void>(`/api/private/sprint/${idSprint}`);
    }

    public close$(idSprint: number): Observable<{ moved: number }> {
        return this.http.post<{ moved: number }>(`/api/private/sprint/${idSprint}/close`, {});
    }

    public assignIssue$(
        idProject: number,
        idIssuePublic: number,
        idSprint: number | null
    ): Observable<void> {
        return this.http.patch<void>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/sprint`,
            { idSprint }
        );
    }

    public stats$(idSprint: number): Observable<SprintStats> {
        return this.http.get<SprintStats>(`/api/private/sprint/${idSprint}/stats`);
    }
}
