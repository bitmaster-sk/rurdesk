import { describe, it, expect } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { UrlSegment } from '@angular/router';
import { of, Observable } from 'rxjs';
import { FirstProjectGuard } from './first-project.guard';
import { ProjectApi } from '../project/api/project.api.service';
import { Project } from '../project/model/project.model';

describe('FirstProjectGuard.canMatch', () => {
    function run(projects: Project[]): boolean {
        TestBed.configureTestingModule({
            providers: [{ provide: ProjectApi, useValue: { load$: () => of(projects) } }]
        });
        const result = TestBed.runInInjectionContext(() =>
            FirstProjectGuard.canMatch({}, [] as UrlSegment[])
        ) as Observable<boolean>;
        let value = undefined as unknown as boolean;
        result.subscribe(v => (value = v));
        return value;
    }

    it('matches (true) when the user has no projects', () => {
        expect(run([])).toBe(true);
    });

    it('does not match (false) when the user has at least one project', () => {
        expect(run([{ idProject: 1, name: 'A', color: '' }])).toBe(false);
    });
});
