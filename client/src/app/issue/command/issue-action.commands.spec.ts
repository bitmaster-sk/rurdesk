import { describe, it, expect, vi } from 'vitest';
import { buildIssueActionCommands, buildIssueCloneCommand } from './issue-action.commands';
import { Issue } from '../model/issue.model';

const issue = {
    idIssuePublic: 5,
    idProject: 1,
    idState: 10,
    idSeverity: null,
    title: 'X'
} as Issue;
const data = {
    states: [{ idState: 11, idProject: 1, name: 'Done' }] as any,
    severities: [{ idSeverity: 2, idProject: 1, title: 'High', color: '#f00' }] as any,
    users: [{ idUser: 9, name: 'Petra', email: 'p@x' }] as any,
    currentUserId: 3
};

const t = (k: string, p?: Record<string, unknown>) => (p ? `${k}:${JSON.stringify(p)}` : k); // fake translator

describe('buildIssueActionCommands', () => {
    it('returns nothing without an issue in context', () => {
        expect(buildIssueActionCommands({ idProject: 1, issue: null }, data, () => {}, t)).toEqual(
            []
        );
    });
    it('sets state via patch', () => {
        const patch = vi.fn();
        buildIssueActionCommands({ idProject: 1, issue }, data, patch, t)
            .find(c => c.id === 'issue.state.11')!
            .run();
        expect(patch).toHaveBeenCalledWith({ idState: 11 });
    });
    it('sets severity by id', () => {
        const patch = vi.fn();
        buildIssueActionCommands({ idProject: 1, issue }, data, patch, t)
            .find(c => c.id === 'issue.sev.2')!
            .run();
        expect(patch).toHaveBeenCalledWith({ idSeverity: 2 });
    });
    it('assigns a member and offers assign-to-me', () => {
        const patch = vi.fn();
        const cmds = buildIssueActionCommands({ idProject: 1, issue }, data, patch, t);
        cmds.find(c => c.id === 'issue.assign.9')!.run();
        expect(patch).toHaveBeenCalledWith({ assignedTo: 9 });
        cmds.find(c => c.id === 'issue.assign.me')!.run();
        expect(patch).toHaveBeenCalledWith({ assignedTo: 3 });
    });
    it('interpolates the data name into the translated title frame', () => {
        const cmd = buildIssueActionCommands({ idProject: 1, issue }, data, () => {}, t).find(
            c => c.id === 'issue.state.11'
        )!;
        expect(cmd.title).toContain('COMMAND.ISSUE.SET_STATE');
        expect(cmd.title).toContain('Done');
    });
    it('omits assign-to-me when there is no current user id', () => {
        const cmds = buildIssueActionCommands(
            { idProject: 1, issue },
            { ...data, currentUserId: null },
            () => {},
            t
        );
        expect(cmds.find(c => c.id === 'issue.assign.me')).toBeUndefined();
    });
});

describe('buildIssueCloneCommand', () => {
    it('offers a clone command on an open issue when the user can create', () => {
        const clone = vi.fn();
        const cmd = buildIssueCloneCommand({ idProject: 1, issue }, true, clone, t)!;
        expect(cmd.id).toBe('issue.clone');
        cmd.run();
        expect(clone).toHaveBeenCalled();
    });
    it('returns null without an open issue or create permission', () => {
        expect(buildIssueCloneCommand({ idProject: 1, issue: null }, true, () => {}, t)).toBeNull();
        expect(buildIssueCloneCommand({ idProject: 1, issue }, false, () => {}, t)).toBeNull();
    });
});
