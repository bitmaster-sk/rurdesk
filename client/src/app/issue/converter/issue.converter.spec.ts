import { IssueConverter } from './issue.converter';
import { Issue } from '../model/issue.model';
import { IssuesFilter } from '../components/filter/issue-filter.entity';

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

describe('IssueConverter.toParams — tags', () => {
    const base: IssuesFilter = { idProject: 1, orderColumn: 'updateAt', orderDirection: 'desc' };

    it('sends the selected tags as one comma list', () => {
        const params = IssueConverter.toParams({ ...base, idsTag: [3, 4] });

        expect(params.get('idsTag')).toBe('3,4');
    });

    it('always sends the unset flag', () => {
        expect(IssueConverter.toParams(base).get('tagUnset')).toBe('false');
        expect(
            IssueConverter.toParams({ ...base, idsTag: [3], tagUnset: true }).get('tagUnset')
        ).toBe('true');
    });
});
