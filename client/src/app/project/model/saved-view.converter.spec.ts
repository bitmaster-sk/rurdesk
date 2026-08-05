import { describe, expect, it } from 'vitest';
import { IssuesFilter } from '../../issue/components/filter/issue-filter.entity';
import { SavedViewConfigConverter } from './saved-view.converter';

describe('SavedViewConfigConverter', () => {
    it('round-trips filter → config → filter with date revival', () => {
        const filter: IssuesFilter = {
            idProject: 5,
            idsState: [1, 2],
            stateUnset: true,
            createAtFrom: new Date('2026-01-01T00:00:00Z'),
            scheduledAtFrom: new Date('2026-07-01T00:00:00Z'),
            orderColumn: 'title',
            orderDirection: 'asc',
            idsIssuePublic: [99],
            idSprint: 7
        };

        const config = SavedViewConfigConverter.toConfig(filter, 'columns');

        expect(config.idsState).toEqual([1, 2]);
        expect(config.stateUnset).toBe(true);
        expect(config.createAtFrom).toBe('2026-01-01T00:00:00.000Z');
        expect(config.kanbanLayout).toBe('columns');
        // contextual / already-rolling fields never persist
        expect('idProject' in config).toBe(false);
        expect('idsIssuePublic' in config).toBe(false);
        expect('idSprint' in config).toBe(false);
        expect('scheduledAtFrom' in config).toBe(false);

        const params = SavedViewConfigConverter.toFilter(config);
        expect(params.idsState).toEqual([1, 2]);
        expect(params.createAtFrom).toBeInstanceOf(Date);
        expect((params.createAtFrom as Date).toISOString()).toBe('2026-01-01T00:00:00.000Z');
        expect(params.orderColumn).toBe('title');
        expect(params.orderDirection).toBe('asc');
    });

    // A window must stay a duration string on both sides so the server keeps resolving it.
    it('passes rolling windows through untouched in both directions', () => {
        const config = SavedViewConfigConverter.toConfig({
            idProject: 1,
            updateAtWithin: '30d',
            createAtWithin: '1d8h6m',
            orderColumn: 'updateAt',
            orderDirection: 'desc'
        });
        expect(config.updateAtWithin).toBe('30d');
        expect(config.createAtWithin).toBe('1d8h6m');

        const params = SavedViewConfigConverter.toFilter(config);
        expect(params.updateAtWithin).toBe('30d');
        expect(params.createAtWithin).toBe('1d8h6m');
    });

    it('drops undefined and null values instead of persisting them', () => {
        const config = SavedViewConfigConverter.toConfig({
            idProject: 1,
            title: undefined,
            idSprint: null,
            orderColumn: 'updateAt',
            orderDirection: 'desc'
        });
        expect('title' in config).toBe(false);
        expect('idSprint' in config).toBe(false);
    });

    // Without them the request omits the sort and hits the server's silent updateAt fallback.
    it('defaults the sort when the config has none', () => {
        const params = SavedViewConfigConverter.toFilter({ v: 1 });
        expect(params.orderColumn).toBe('updateAt');
        expect(params.orderDirection).toBe('desc');
        expect(params.idsState).toBeUndefined();
    });

    it('drops a date it cannot parse instead of passing it on', () => {
        const params = SavedViewConfigConverter.toFilter({
            v: 1,
            createAtFrom: 'tomorrow',
            updateAtFrom: '2026-01-01T00:00:00.000Z'
        });

        expect('createAtFrom' in params).toBe(false);
        expect(params.updateAtFrom).toBeInstanceOf(Date);
    });

    it('drops an id list that is not an array', () => {
        const params = SavedViewConfigConverter.toFilter({
            v: 1,
            idsState: 7 as unknown as number[],
            idsSeverity: [3]
        });

        expect('idsState' in params).toBe(false);
        expect(params.idsSeverity).toEqual([3]);
    });

    it('never leaks the schema version or the kanban layout into filter params', () => {
        const params = SavedViewConfigConverter.toFilter({
            v: 1,
            kanbanLayout: 'swimlane',
            idsState: [3]
        });
        expect('v' in params).toBe(false);
        expect('kanbanLayout' in params).toBe(false);
        expect(params.idsState).toEqual([3]);
    });
});
