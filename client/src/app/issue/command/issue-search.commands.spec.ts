import { describe, it, expect, vi } from 'vitest';
import { IssueSearchCommands } from './issue-search.commands';
import { Issue } from '../model/issue.model';
import { Fixtures } from 'src/testing/fixtures';

const t = (k: string) => k;

describe('IssueSearchCommands.build', () => {
    it('maps issues to #-prefixed jump commands with numeric keywords + bare-token completion', () => {
        const nav = vi.fn();
        const issue: Issue = Fixtures.issue({
            idIssuePublic: 428,
            idProject: 4,
            title: 'Login'
        });
        const cmds = IssueSearchCommands.build({ idProject: 4, issue: null }, [issue], nav, t);
        expect(cmds[0].title).toBe('#428 Login');
        expect(cmds[0].keywords).toBe('428'); // bare number → exact-ID ranking tier fires on real data
        expect(cmds[0].completion).toBe('#428');
        cmds[0].run();
        expect(nav).toHaveBeenCalledWith(['/project', 4, 'issue', 428]);
    });
});
