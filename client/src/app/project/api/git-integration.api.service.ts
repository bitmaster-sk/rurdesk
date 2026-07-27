import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import {
    CreateGitIntegrationReq,
    GitIntegrationRes,
    UpdateGitIntegrationReq
} from '../model/git-integration.model';

@Injectable({ providedIn: 'root' })
export class GitIntegrationApi {
    private readonly http = inject(HttpClient);

    public list$(idProject: number): Observable<GitIntegrationRes[]> {
        return this.http.get<GitIntegrationRes[]>(
            `/api/private/project/${idProject}/git-integration`
        );
    }

    public get$(idProject: number, idGitIntegration: number): Observable<GitIntegrationRes> {
        return this.http.get<GitIntegrationRes>(
            `/api/private/project/${idProject}/git-integration/${idGitIntegration}`
        );
    }

    public create$(idProject: number, req: CreateGitIntegrationReq): Observable<GitIntegrationRes> {
        return this.http.post<GitIntegrationRes>(
            `/api/private/project/${idProject}/git-integration`,
            req
        );
    }

    public update$(
        idProject: number,
        idGitIntegration: number,
        req: UpdateGitIntegrationReq
    ): Observable<GitIntegrationRes> {
        return this.http.put<GitIntegrationRes>(
            `/api/private/project/${idProject}/git-integration/${idGitIntegration}`,
            req
        );
    }

    public delete$(idProject: number, idGitIntegration: number): Observable<void> {
        return this.http.delete<void>(
            `/api/private/project/${idProject}/git-integration/${idGitIntegration}`
        );
    }
}
