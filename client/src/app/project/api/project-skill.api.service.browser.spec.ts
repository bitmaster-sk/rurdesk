import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { AgentStage } from 'src/app/agent/model/agent-stage.enum';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ProjectSkillApi } from './project-skill.api.service';

describe('ProjectSkillApi', () => {
    let api: ProjectSkillApi;
    let http: HttpTestingController;

    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [provideHttpClient(), provideHttpClientTesting()]
        });
        api = TestBed.inject(ProjectSkillApi);
        http = TestBed.inject(HttpTestingController);
    });

    afterEach(() => http.verify());

    it('loads the matrix of one project', () => {
        api.load$(3).subscribe();

        const request = http.expectOne('/api/private/project/3/skills');
        expect(request.request.method).toBe('GET');
        request.flush([]);
    });

    it('replaces the whole matrix in one call', () => {
        api.replace$(3, [{ idSkill: 9, stage: AgentStage.Design }]).subscribe();

        const request = http.expectOne('/api/private/project/3/skills');
        expect(request.request.method).toBe('PUT');
        expect(request.request.body).toEqual([{ idSkill: 9, stage: 'design' }]);
        request.flush([]);
    });
});
