import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { QualityReport } from '../model/quality.model';
import { silentErrors } from 'src/app/core/http-error-context';

@Injectable({ providedIn: 'root' })
export class QualityApi {
    private readonly http = inject(HttpClient);

    public preview$(
        idProject: number,
        title: string,
        description: string
    ): Observable<QualityReport> {
        return this.http.post<QualityReport>(`/api/private/project/${idProject}/quality`, {
            title,
            description
        });
    }

    public check$(
        idProject: number,
        idIssuePublic: number,
        title: string,
        description: string
    ): Observable<QualityReport> {
        return this.http.post<QualityReport>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/quality`,
            { title, description }
        );
    }

    // 404 is the normal answer for an issue nobody has quality-checked yet, so
    // this must not raise the global error toast.
    public getQuality$(idProject: number, idIssuePublic: number): Observable<QualityReport> {
        return this.http.get<QualityReport>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/quality`,
            { context: silentErrors() }
        );
    }
}
