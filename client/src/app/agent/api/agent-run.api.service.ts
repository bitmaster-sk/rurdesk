import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { AgentRun, RunStats } from '../model/agent-run.model';

@Injectable({ providedIn: 'root' })
export class AgentRunApi {
    private readonly http = inject(HttpClient);

    public getRunByIssue$(idProject: number, idIssuePublic: number): Observable<AgentRun | null> {
        return this.http.get<AgentRun | null>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/agent/run`
        );
    }

    public getRunsByProject$(idProject: number): Observable<AgentRun[]> {
        return this.http.get<AgentRun[]>(`/api/private/project/${idProject}/agent/runs`);
    }

    public approve$(idRun: number, mockupRef?: string): Observable<AgentRun> {
        const body = mockupRef ? { mockupRef } : {};
        return this.http.post<AgentRun>(`/api/private/agent/run/${idRun}/approve`, body);
    }

    public cancel$(idRun: number): Observable<AgentRun> {
        return this.http.post<AgentRun>(`/api/private/agent/run/${idRun}/cancel`, {});
    }

    public continue$(idRun: number): Observable<{ idRun: number; nextStage: string }> {
        return this.http.post<{ idRun: number; nextStage: string }>(
            `/api/private/agent/run/${idRun}/continue`,
            {}
        );
    }

    public restart$(idRun: number): Observable<{ oldIdRun: number; newIdRun: number }> {
        return this.http.post<{ oldIdRun: number; newIdRun: number }>(
            `/api/private/agent/run/${idRun}/restart`,
            {}
        );
    }

    public stats$(idRun: number): Observable<RunStats> {
        return this.http.get<RunStats>(`/api/private/agent/run/${idRun}/stats`);
    }
}
