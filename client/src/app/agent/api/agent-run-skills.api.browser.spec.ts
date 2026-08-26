import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { AgentStage } from '../model/agent-stage.enum';
import { AgentRunApi } from './agent-run.api.service';

describe('AgentRunApi skills endpoints', () => {
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

    it('reads the per-run skills', () => {
        api.getAgentRunSkills$(5).subscribe();

        const request = http.expectOne('/api/private/agent/run/5/skills');
        expect(request.request.method).toBe('GET');
        request.flush({ stages: [] });
    });

    it('patches one stage with the ids it should run with', () => {
        api.patchAgentRunSkills$(5, AgentStage.Implementation, [1, 2]).subscribe();

        const request = http.expectOne('/api/private/agent/run/5/skills');
        expect(request.request.method).toBe('PATCH');
        expect(request.request.body).toEqual({ stage: 'implementation', idsSkill: [1, 2] });
        request.flush({ stages: [] });
    });

    it('reads the bot workload overview per project', () => {
        api.agentsOverview$(3).subscribe();

        const request = http.expectOne('/api/private/project/3/agents/overview');
        expect(request.request.method).toBe('GET');
        request.flush([]);
    });

    it('assigns a bot with explicit per-stage skills', () => {
        api.assignAgent$(3, 42, {
            idUserBot: 8,
            idsSkillByStage: { implementation: [1] }
        }).subscribe();

        const request = http.expectOne('/api/private/project/3/issue/42/assign-agent');
        expect(request.request.method).toBe('POST');
        expect(request.request.body).toEqual({
            idUserBot: 8,
            idsSkillByStage: { implementation: [1] }
        });
        request.flush({});
    });
});
