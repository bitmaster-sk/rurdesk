import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { MrDiff, MrStatus } from '../../project/model/git-integration.model';

@Injectable({ providedIn: 'root' })
export class MrDiffApi {
    private readonly http = inject(HttpClient);

    public getDiff$(idProject: number, idGitIntegration: number, mrId: string): Observable<MrDiff> {
        return this.http.get<MrDiff>(
            `/api/private/project/${idProject}/git-integration/${idGitIntegration}/mr/${mrId}/diff`
        );
    }

    public getStatus$(
        idProject: number,
        idGitIntegration: number,
        mrId: string
    ): Observable<MrStatus> {
        return this.http.get<MrStatus>(
            `/api/private/project/${idProject}/git-integration/${idGitIntegration}/mr/${mrId}/status`
        );
    }
}
