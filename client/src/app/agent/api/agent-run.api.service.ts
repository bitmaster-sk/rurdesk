import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import {
    AgentRunStagePlanEntry,
    AgentRunStagePlanRes,
    UpdateAgentRunStagePlanEntryReq
} from '../model/agent-run-stage-plan.model';
import { CreateAgentRunReq } from '../model/agent-run.model';
import { AgentOverview } from '../model/agent-overview.model';
import { RunKind } from '../model/run-kind.model';
import { AgentRun, RunStats, UpdateAgentRunWorkflowReq } from '../model/agent-run.model';
import { AgentRunWikiRead } from '../model/agent-run-wiki-read.model';

@Injectable({ providedIn: 'root' })
export class AgentRunApi {
    private readonly http = inject(HttpClient);

    public loadByIssue$(idProject: number, idIssuePublic: number): Observable<AgentRun | null> {
        return this.http.get<AgentRun | null>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/agent/run`
        );
    }

    public loadByProject$(idProject: number): Observable<AgentRun[]> {
        return this.http.get<AgentRun[]>(`/api/private/project/${idProject}/agent/runs`);
    }

    public loadOne$(idRun: number): Observable<AgentRun> {
        return this.http.get<AgentRun>(`/api/private/agent/run/${idRun}`);
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

    public loadStats$(idRun: number): Observable<RunStats> {
        return this.http.get<RunStats>(`/api/private/agent/run/${idRun}/stats`);
    }

    public loadWikiReads$(idRun: number): Observable<AgentRunWikiRead[]> {
        return this.http.get<AgentRunWikiRead[]>(`/api/private/agent/run/${idRun}/wiki-read`);
    }

    public loadStagePlan$(idRun: number): Observable<AgentRunStagePlanRes> {
        return this.http.get<AgentRunStagePlanRes>(`/api/private/agent/run/${idRun}/stage-plan`);
    }

    public updateStagePlanEntry$(
        idRun: number,
        entry: AgentRunStagePlanEntry
    ): Observable<AgentRunStagePlanRes> {
        const body: UpdateAgentRunStagePlanEntryReq = {
            stage: entry.name,
            idsSkill: entry.idsSkill
        };
        if (entry.model !== null) {
            body.model = entry.model;
        }
        if (entry.maxTurns !== null) {
            body.maxTurns = entry.maxTurns;
        }
        return this.http.patch<AgentRunStagePlanRes>(
            `/api/private/agent/run/${idRun}/stage-plan`,
            body
        );
    }

    public updateWorkflow$(idRun: number, isShortWorkflow: boolean): Observable<AgentRun> {
        const body: UpdateAgentRunWorkflowReq = { isShortWorkflow };
        return this.http.patch<AgentRun>(`/api/private/agent/run/${idRun}/workflow`, body);
    }

    public loadRunKinds$(): Observable<RunKind[]> {
        return this.http.get<RunKind[]>('/api/private/agent/run-kinds');
    }

    public loadAgentsOverview$(idProject: number): Observable<AgentOverview[]> {
        return this.http.get<AgentOverview[]>(`/api/private/project/${idProject}/agents/overview`);
    }

    public assignAgent$(
        idProject: number,
        idIssuePublic: number,
        body: CreateAgentRunReq
    ): Observable<AgentRun> {
        return this.http.post<AgentRun>(
            `/api/private/project/${idProject}/issue/${idIssuePublic}/assign-agent`,
            body
        );
    }
}
