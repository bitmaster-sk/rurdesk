import { describe, expect, it } from 'vitest';
import { ProjectSwitchTarget } from './project-switch-target';

describe('ProjectSwitchTarget.url', () => {
    it('keeps the issue view mode', () => {
        expect(ProjectSwitchTarget.url('/project/1/issue/view/kanban', 2)).toEqual([
            '/project',
            2,
            'issue',
            'view',
            'kanban'
        ]);
    });

    it('keeps the section but drops a child id of the old project', () => {
        expect(ProjectSwitchTarget.url('/project/1/wiki/42', 2)).toEqual(['/project', 2, 'wiki']);
        expect(ProjectSwitchTarget.url('/project/1/release?tab=done', 2)).toEqual([
            '/project',
            2,
            'release'
        ]);
        expect(ProjectSwitchTarget.url('/project/1/settings', 2)).toEqual([
            '/project',
            2,
            'settings'
        ]);
    });

    it('opens the overview from an issue detail', () => {
        expect(ProjectSwitchTarget.url('/project/1/issue/123', 2)).toEqual(['/project', 2, 'view']);
    });

    it('opens the overview from a page outside the project', () => {
        expect(ProjectSwitchTarget.url('/message/1/team/view', 2)).toEqual(['/project', 2, 'view']);
        expect(ProjectSwitchTarget.url('/agent-chat', 2)).toEqual(['/project', 2, 'view']);
    });
});
