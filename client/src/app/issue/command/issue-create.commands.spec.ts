import { describe, it, expect, vi } from 'vitest';
import { IssueCreateCommands } from './issue-create.commands';

const t = (k: string, p?: Record<string, unknown>) => (p ? `${k}:${JSON.stringify(p)}` : k);

describe('IssueCreateCommands.build', () => {
    it('returns a create command carrying the typed title (interpolated into the translated frame)', () => {
        const create = vi.fn();
        const cmd = IssueCreateCommands.build(
            'login bug',
            { idProject: 4, issue: null },
            true,
            create,
            t
        )!;
        expect(cmd.id).toBe('issue.create');
        expect(cmd.title).toContain('login bug');
        cmd.run();
        expect(create).toHaveBeenCalledWith('login bug');
    });
    it('returns null without a query, project, or create permission', () => {
        expect(
            IssueCreateCommands.build('', { idProject: 4, issue: null }, true, () => {}, t)
        ).toBeNull();
        expect(
            IssueCreateCommands.build('x', { idProject: null, issue: null }, true, () => {}, t)
        ).toBeNull();
        expect(
            IssueCreateCommands.build('x', { idProject: 4, issue: null }, false, () => {}, t)
        ).toBeNull();
    });
});
