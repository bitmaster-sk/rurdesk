import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { IssueSeverity } from '../model/issue-severity.model';
import { SeverityUsage } from '../model/severity-usage.model';

@Injectable({
    providedIn: 'root'
})
export class SeverityApi {
    private http = inject(HttpClient);

    public load$(): Observable<IssueSeverity[]> {
        return this.http.get<IssueSeverity[]>(`/api/private/severity`);
    }

    public insert$(severity: IssueSeverity): Observable<IssueSeverity> {
        return this.http.post<IssueSeverity>(`/api/private/severity`, severity);
    }

    public update$(severity: IssueSeverity): Observable<IssueSeverity> {
        return this.http.patch<IssueSeverity>(
            `/api/private/severity/${severity.idSeverity}`,
            severity
        );
    }

    public usage$(idProject: number, idSeverity: number): Observable<SeverityUsage> {
        return this.http.get<SeverityUsage>(
            `/api/private/severity/${idSeverity}/project/${idProject}/usage`
        );
    }

    public delete$(
        idProject: number,
        idSeverity: number,
        intent?: { migrateTo: number | null }
    ): Observable<void> {
        let params = new HttpParams();
        if (intent) {
            params = params.set(
                'migrateTo',
                intent.migrateTo === null ? 'null' : String(intent.migrateTo)
            );
        }
        return this.http.delete<void>(`/api/private/severity/${idSeverity}/project/${idProject}`, {
            params
        });
    }
}
