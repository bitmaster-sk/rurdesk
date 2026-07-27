import { describe, it, expect, vi } from 'vitest';
import { buildIssueJumpCommands } from './issue-search.commands';
import { Issue } from '../model/issue.model';

const t = (k: string) => k;

describe('buildIssueJumpCommands', () => {
    it('maps issues to #-prefixed jump commands with numeric keywords + bare-token completion', () => {
        const nav = vi.fn();
        const cmds = buildIssueJumpCommands(
            { idProject: 4, issue: null },
            [{ idIssuePublic: 428, idProject: 4, title: 'Login' } as Issue],
            nav,
            t
        );
        expect(cmds[0].title).toBe('#428 Login');
        expect(cmds[0].keywords).toBe('428'); // bare number → exact-ID ranking tier fires on real data
        expect(cmds[0].completion).toBe('#428');
        cmds[0].run();
        expect(nav).toHaveBeenCalledWith(['/project', 4, 'issue', 428]);
    });
});
