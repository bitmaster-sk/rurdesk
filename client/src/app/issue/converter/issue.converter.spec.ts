import { IssueConverter } from './issue.converter';
import { Issue } from '../model/issue.model';

describe('IssueConverter.toIssue', () => {
    it('converts scheduledAt when present', () => {
        const out = IssueConverter.toIssue({
            createAt: '2026-01-01T00:00:00Z',
            updateAt: '2026-01-01T00:00:00Z',
            scheduledAt: '2026-03-03T00:00:00Z'
        } as unknown as Issue);
        expect(out.scheduledAt).toBeInstanceOf(Date);
    });

    it('leaves scheduledAt null when absent', () => {
        const out = IssueConverter.toIssue({
            createAt: '2026-01-01T00:00:00Z',
            updateAt: '2026-01-01T00:00:00Z',
            scheduledAt: null
        } as unknown as Issue);
        expect(out.scheduledAt).toBeNull();
    });
});
