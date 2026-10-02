import { IssuesFilter } from '../components/filter/issue-filter.entity';

const DATE_FIELDS = [
    'createAtFrom',
    'createAtTo',
    'updateAtFrom',
    'updateAtTo',
    'scheduledAtFrom',
    'scheduledAtTo'
] as const;

export abstract class IssueListFilterConverter {
    public static toStorage(filter: IssuesFilter): string {
        return JSON.stringify(filter);
    }

    public static toFilter(raw: string | null): IssuesFilter | null {
        if (raw === null) {
            return null;
        }
        let parsed: unknown;
        try {
            parsed = JSON.parse(raw);
        } catch {
            return null;
        }
        if (!IssueListFilterConverter.isFilter(parsed)) {
            return null;
        }
        const filter: IssuesFilter = { ...parsed };
        for (const field of DATE_FIELDS) {
            const value: unknown = filter[field];
            filter[field] = typeof value === 'string' ? new Date(value) : (filter[field] ?? null);
        }
        return filter;
    }

    private static isFilter(value: unknown): value is IssuesFilter {
        if (typeof value !== 'object' || value === null) {
            return false;
        }
        const candidate = value as Partial<IssuesFilter>;
        return (
            typeof candidate.idProject === 'number' &&
            typeof candidate.orderColumn === 'string' &&
            (candidate.orderDirection === 'asc' || candidate.orderDirection === 'desc')
        );
    }
}
