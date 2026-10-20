import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { RequestContext } from 'src/app/core/request-context';
import { PlantUmlRenderReq, PlantUmlRenderRes } from './model/plantuml.model';

@Injectable({ providedIn: 'root' })
export class PlantUmlApi {
    private readonly http = inject(HttpClient);

    public render$(source: string): Observable<PlantUmlRenderRes> {
        const req: PlantUmlRenderReq = { source };
        return this.http.post<PlantUmlRenderRes>('/api/private/plantuml/svg', req, {
            context: RequestContext.disableErrorToast()
        });
    }
}
