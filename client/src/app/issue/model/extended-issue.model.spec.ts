import { IssueGuard } from './extended-issue.model';
import { Issue } from './issue.model';

function makeIssue(scheduledAt: Date | null | undefined): Issue {
    return {
        idIssue: 1,
        idIssuePublic: 1,
        idProject: 1,
        idState: null,
        idSeverity: null,
        title: 'T',
        description: '',
        tracked: 0,
        scheduledAt
    };
}

describe('IssueGuard.isScheduled', () => {
    it('returns false for null', () => {
        expect(IssueGuard.isScheduled(makeIssue(null))).toBe(false);
    });

    it('returns false for undefined', () => {
        expect(IssueGuard.isScheduled(makeIssue(undefined))).toBe(false);
    });

    it('returns false for Invalid Date', () => {
        expect(IssueGuard.isScheduled(makeIssue(new Date('not-a-date')))).toBe(false);
    });

    it('returns true for a valid Date', () => {
        expect(IssueGuard.isScheduled(makeIssue(new Date('2026-01-01T00:00:00Z')))).toBe(true);
    });
});
