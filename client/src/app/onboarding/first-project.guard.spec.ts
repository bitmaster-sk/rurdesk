import { describe, it, expect } from 'vitest';
import { hasNoProjects } from './first-project.guard';
import { Project } from '../project/model/project.model';

describe('hasNoProjects', () => {
    it('is true when the user has no projects', () => {
        expect(hasNoProjects([])).toBe(true);
    });

    it('is false when the user has at least one project', () => {
        expect(hasNoProjects([{ idProject: 1, name: 'A', color: '' } as Project])).toBe(false);
    });
});
