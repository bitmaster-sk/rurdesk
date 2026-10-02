import { IssuesFilter } from '../components/filter/issue-filter.entity';
import { IssueListFilterConverter } from './issue-list-filter.converter';

describe('IssueListFilterConverter', () => {
    it('round-trips a filter and brings its dates back as Date objects', () => {
        const filter: IssuesFilter = {
            idProject: 3,
            orderColumn: 'title',
            orderDirection: 'asc',
            title: 'login',
            idsState: [1, 2],
            createAtFrom: new Date('2026-09-01T00:00:00.000Z'),
            createAtTo: null,
            updateAtWithin: '7d'
        };

        const restored = IssueListFilterConverter.toFilter(
            IssueListFilterConverter.toStorage(filter)
        );

        expect(restored).toEqual({
            ...filter,
            updateAtFrom: null,
            updateAtTo: null,
            scheduledAtFrom: null,
            scheduledAtTo: null
        });
        expect(restored?.createAtFrom).toBeInstanceOf(Date);
    });

    it('returns null for nothing stored', () => {
        expect(IssueListFilterConverter.toFilter(null)).toBeNull();
    });

    it('returns null for a corrupted entry', () => {
        expect(IssueListFilterConverter.toFilter('{not json')).toBeNull();
    });

    it('returns null for JSON that is not a filter', () => {
        expect(IssueListFilterConverter.toFilter('{"title":"x"}')).toBeNull();
        expect(IssueListFilterConverter.toFilter('[1,2]')).toBeNull();
    });
});
