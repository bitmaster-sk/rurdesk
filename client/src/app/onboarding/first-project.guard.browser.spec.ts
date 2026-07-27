import { describe, it, expect } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { Route, UrlSegment } from '@angular/router';
import { of, Observable } from 'rxjs';
import { firstProjectGuard } from './first-project.guard';
import { ProjectService } from '../project/project.service';
import { Project } from '../project/model/project.model';

describe('firstProjectGuard', () => {
    function run(projects: Project[]): boolean {
        TestBed.configureTestingModule({
            providers: [{ provide: ProjectService, useValue: { loadProjects: () => of(projects) } }]
        });
        const result = TestBed.runInInjectionContext(() =>
            firstProjectGuard({} as Route, [] as UrlSegment[])
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
