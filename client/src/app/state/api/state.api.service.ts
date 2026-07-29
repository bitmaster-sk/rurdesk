import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { IssueState } from '../model/issue-state.model';
import { StateUsage } from '../model/state-usage.model';

@Injectable({
    providedIn: 'root'
})
export class StateApi {
    private http = inject(HttpClient);

    public load$(): Observable<IssueState[]> {
        return this.http.get<IssueState[]>(`/api/private/state`);
    }

    public insert$(state: IssueState): Observable<IssueState> {
        return this.http.post<IssueState>(`/api/private/state`, state);
    }

    public update$(state: IssueState): Observable<IssueState> {
        return this.http.patch<IssueState>(`/api/private/state/${state.idState}`, state);
    }

    public usage$(idProject: number, idState: number): Observable<StateUsage> {
        return this.http.get<StateUsage>(
            `/api/private/state/${idState}/project/${idProject}/usage`
        );
    }

    public delete$(
        idProject: number,
        idState: number,
        intent?: { migrateTo: number | null }
    ): Observable<void> {
        let params = new HttpParams();
        if (intent) {
            params = params.set(
                'migrateTo',
                intent.migrateTo === null ? 'null' : String(intent.migrateTo)
            );
        }
        return this.http.delete<void>(`/api/private/state/${idState}/project/${idProject}`, {
            params
        });
    }
}
