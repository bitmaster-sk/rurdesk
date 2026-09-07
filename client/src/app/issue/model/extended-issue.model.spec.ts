import { IssueGuard } from './extended-issue.model';
import { Issue } from './issue.model';
import { Fixtures } from 'src/testing/fixtures';

function makeIssue(scheduledAt: Date | null | undefined): Issue {
    return Fixtures.issue({ title: 'T', scheduledAt });
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
