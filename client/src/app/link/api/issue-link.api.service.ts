import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { IssueLinkInfo, IssueLinkOption, IssueLinkRef } from '../model/issue-link.model';

@Injectable({ providedIn: 'root' })
export class IssueLinkApi {
    private readonly http = inject(HttpClient);

    public load$(query: string, idProject: number | null): Observable<IssueLinkOption[]> {
        let params = new HttpParams().set('q', query);
        if (idProject !== null) {
            params = params.set('idProject', idProject);
        }
        return this.http.get<IssueLinkOption[]>('/api/private/issue-link/search', { params });
    }

    public loadInfo$(refs: IssueLinkRef[]): Observable<IssueLinkInfo> {
        return this.http.post<IssueLinkInfo>('/api/private/issue-link/info', { refs });
    }
}
