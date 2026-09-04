import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { AgentThinkingRes } from '../model/agent-thinking.model';

@Injectable({ providedIn: 'root' })
export class AgentThinkingApi {
    private readonly http = inject(HttpClient);

    public loadStageThinking$(idRun: number, stage: string): Observable<AgentThinkingRes> {
        return this.http.get<AgentThinkingRes>(`/api/private/agent/run/${idRun}/thinking`, {
            params: { stage }
        });
    }
}
