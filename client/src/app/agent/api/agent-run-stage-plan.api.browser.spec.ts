import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { AgentStage } from '../model/agent-stage.enum';
import { AgentRunApi } from './agent-run.api.service';

describe('AgentRunApi stage plan endpoints', () => {
    let api: AgentRunApi;
    let http: HttpTestingController;

    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [provideHttpClient(), provideHttpClientTesting()]
        });
        api = TestBed.inject(AgentRunApi);
        http = TestBed.inject(HttpTestingController);
    });

    afterEach(() => http.verify());

    it('reads the stage plan of a run', () => {
        api.loadStagePlan$(5).subscribe();

        const request = http.expectOne('/api/private/agent/run/5/stage-plan');
        expect(request.request.method).toBe('GET');
        request.flush({ stages: [] });
    });

    it('patches one stage with the skills, model and turns it should run with', () => {
        api.updateStagePlanEntry$(5, {
            name: AgentStage.Implementation,
            idsSkill: [1, 2],
            model: 'openai/b',
            maxTurns: 90,
            skip: false,
            dispatched: false
        }).subscribe();

        const request = http.expectOne('/api/private/agent/run/5/stage-plan');
        expect(request.request.method).toBe('PATCH');
        expect(request.request.body).toEqual({
            stage: 'implementation',
            idsSkill: [1, 2],
            model: 'openai/b',
            maxTurns: 90
        });
        request.flush({ stages: [] });
    });

    it('leaves out the model and turns a stage inherits', () => {
        api.updateStagePlanEntry$(5, {
            name: AgentStage.Design,
            idsSkill: [],
            model: null,
            maxTurns: null,
            skip: false,
            dispatched: false
        }).subscribe();

        const request = http.expectOne('/api/private/agent/run/5/stage-plan');
        expect(request.request.body).toEqual({ stage: 'design', idsSkill: [] });
        request.flush({ stages: [] });
    });

    it('reads the agent workload overview per project', () => {
        api.loadAgentsOverview$(3).subscribe();

        const request = http.expectOne('/api/private/project/3/agents/overview');
        expect(request.request.method).toBe('GET');
        request.flush([]);
    });

    it('assigns an agent with explicit per-stage skills and workflow length', () => {
        api.assignAgent$(3, 42, {
            idUserAgent: 8,
            idsSkillByStage: { implementation: [1] },
            isShortWorkflow: true
        }).subscribe();

        const request = http.expectOne('/api/private/project/3/issue/42/assign-agent');
        expect(request.request.method).toBe('POST');
        expect(request.request.body).toEqual({
            idUserAgent: 8,
            idsSkillByStage: { implementation: [1] },
            isShortWorkflow: true
        });
        request.flush({});
    });

    it('patches the workflow length of a run', () => {
        api.updateWorkflow$(55, true).subscribe();

        const request = http.expectOne('/api/private/agent/run/55/workflow');
        expect(request.request.method).toBe('PATCH');
        expect(request.request.body).toEqual({ isShortWorkflow: true });
        request.flush({});
    });
});
