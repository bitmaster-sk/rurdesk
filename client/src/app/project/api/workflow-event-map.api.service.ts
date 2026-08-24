import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import {
    WorkflowEventMapping,
    WorkflowEventMappingEntry
} from '../model/workflow-event-mapping.model';
import { silentErrors } from 'src/app/core/http-error-context';

@Injectable({ providedIn: 'root' })
export class WorkflowEventMapApi {
    private readonly http = inject(HttpClient);

    public load$(idProject: number): Observable<WorkflowEventMapping[]> {
        return this.http.get<WorkflowEventMapping[]>(
            `/api/private/project/${idProject}/workflow-event-state-map`
        );
    }

    public replace$(
        idProject: number,
        mappings: WorkflowEventMappingEntry[]
    ): Observable<WorkflowEventMapping[]> {
        return this.http.put<WorkflowEventMapping[]>(
            `/api/private/project/${idProject}/workflow-event-state-map`,
            { mappings },
            { context: silentErrors() }
        );
    }
}
