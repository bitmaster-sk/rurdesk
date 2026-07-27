import { describe, it, expect, vi } from 'vitest';
import { buildCreateFromQuery } from './create-issue.commands';

const t = (k: string, p?: Record<string, unknown>) => (p ? `${k}:${JSON.stringify(p)}` : k);

describe('buildCreateFromQuery', () => {
    it('returns a create command carrying the typed title (interpolated into the translated frame)', () => {
        const create = vi.fn();
        const cmd = buildCreateFromQuery(
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
            buildCreateFromQuery('', { idProject: 4, issue: null }, true, () => {}, t)
        ).toBeNull();
        expect(
            buildCreateFromQuery('x', { idProject: null, issue: null }, true, () => {}, t)
        ).toBeNull();
        expect(
            buildCreateFromQuery('x', { idProject: 4, issue: null }, false, () => {}, t)
        ).toBeNull();
    });
});
