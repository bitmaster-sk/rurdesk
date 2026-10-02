import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';
import { RequestContext } from '../../core/request-context';
import { MrDiff, MrStatus } from '../../project/model/git-integration.model';

@Injectable({ providedIn: 'root' })
export class MrDiffApi {
    private readonly http = inject(HttpClient);

    public load$(idProject: number, idGitIntegration: number, mrId: string): Observable<MrDiff> {
        return this.http.get<MrDiff>(
            `/api/private/project/${idProject}/git-integration/${idGitIntegration}/mr/${mrId}/diff`
        );
    }

    public loadStatus$(
        idProject: number,
        idGitIntegration: number,
        mrId: string
    ): Observable<MrStatus> {
        return this.http.get<MrStatus>(
            `/api/private/project/${idProject}/git-integration/${idGitIntegration}/mr/${mrId}/status`
        );
    }

    public loadFileContent$(
        idProject: number,
        idGitIntegration: number,
        mrId: string,
        path: string,
        ref: string
    ): Observable<string[]> {
        return this.http
            .get<{ lines: string[] }>(
                `/api/private/project/${idProject}/git-integration/${idGitIntegration}/mr/${mrId}/file-content`,
                { params: { path, ref }, context: RequestContext.disableErrorToast() }
            )
            .pipe(map(res => res.lines));
    }
}
