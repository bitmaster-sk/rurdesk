import { describe, it, expect } from 'vitest';
import { FirstProjectGuard } from './first-project.guard';

describe('FirstProjectGuard.hasNoProjects', () => {
    it('is true when the user has no projects', () => {
        expect(FirstProjectGuard.hasNoProjects([])).toBe(true);
    });

    it('is false when the user has at least one project', () => {
        expect(FirstProjectGuard.hasNoProjects([{ idProject: 1, name: 'A', color: '' }])).toBe(
            false
        );
    });
});
