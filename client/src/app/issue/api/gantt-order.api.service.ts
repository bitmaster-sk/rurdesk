import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { silentErrors } from 'src/app/core/http-error-context';

export interface GanttReorderRequest {
    movedId: number;
    order: number[];
}

@Injectable({ providedIn: 'root' })
export class GanttOrderApi {
    private readonly http = inject(HttpClient);

    public reorder$(idProject: number, body: GanttReorderRequest): Observable<void> {
        return this.http.put<void>(`/api/private/project/${idProject}/gantt-order`, body, {
            context: silentErrors()
        });
    }
}
