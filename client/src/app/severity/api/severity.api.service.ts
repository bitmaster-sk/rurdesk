import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { IssueSeverity } from '../model/issue-severity.model';

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

    public delete$(idProject: number, idSeverity: number): Observable<void> {
        return this.http.delete<void>(`/api/private/severity/${idSeverity}/project/${idProject}`);
    }
}
