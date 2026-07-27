import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { IssueState } from '../model/issue-state.model';

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

    public delete$(idProject: number, idState: number): Observable<void> {
        return this.http.delete<void>(`/api/private/state/${idState}/project/${idProject}`);
    }
}
