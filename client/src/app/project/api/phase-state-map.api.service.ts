import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { PhaseStateMapping, PhaseStateMappingEntry } from '../model/phase-state-mapping.model';
import { silentErrors } from 'src/app/core/http-error-context';

@Injectable({ providedIn: 'root' })
export class PhaseStateMapApi {
    private readonly http = inject(HttpClient);

    public load$(idProject: number): Observable<PhaseStateMapping[]> {
        return this.http.get<PhaseStateMapping[]>(
            `/api/private/project/${idProject}/agent-phase-state-map`
        );
    }

    public replace$(
        idProject: number,
        mappings: PhaseStateMappingEntry[]
    ): Observable<PhaseStateMapping[]> {
        return this.http.put<PhaseStateMapping[]>(
            `/api/private/project/${idProject}/agent-phase-state-map`,
            { mappings },

            { context: silentErrors() }
        );
    }
}
