import { IssueFilterStore } from './issue-filter.store';
import { IssuesFilter, IssuesFilterParams, IssuesOrderParams } from './issue-filter.entity';

function latest<T>(obs: { subscribe: (fn: (v: T) => void) => unknown }): T | undefined {
    let v: T | undefined;
    obs.subscribe(x => (v = x));
    return v;
}

describe('IssueFilterStore', () => {
    let store: IssueFilterStore;
    beforeEach(() => (store = new IssueFilterStore()));

    it('does not emit an actual filter until one is set', () => {
        expect(latest(store.actualFilter$)).toBeUndefined();
    });

    it('setFilter merges params onto the current filter', () => {
        store.setFilter({ title: 'login' } as IssuesFilterParams);
        store.setFilter({ stateUnset: true } as IssuesFilterParams);
        expect(latest(store.actualFilter$)).toMatchObject({ title: 'login', stateUnset: true });
    });

    it('setOrder merges order params without dropping filter params', () => {
        store.setFilter({ title: 'login' } as IssuesFilterParams);
        store.setOrder({ orderColumn: 'updateAt', orderDirection: 'desc' } as IssuesOrderParams);
        expect(latest(store.actualFilter$)).toMatchObject({
            title: 'login',
            orderColumn: 'updateAt',
            orderDirection: 'desc'
        });
    });

    it('setSprint merges idSprint and clears it with null', () => {
        store.setInitialFilter({ idProject: 1 } as IssuesFilter);
        store.setSprint(9);
        expect(latest(store.actualFilter$)).toMatchObject({ idProject: 1, idSprint: 9 });
        store.setSprint(null);
        expect(latest(store.actualFilter$)?.idSprint).toBeNull();
    });

    it('setInitialFilter emits on both actual and initial streams', () => {
        const f = { idProject: 1 } as IssuesFilter;
        store.setInitialFilter(f);
        expect(latest(store.initialFilter$)).toMatchObject({ idProject: 1 });
        expect(latest(store.actualFilter$)).toMatchObject({ idProject: 1 });
    });

    it('getFilter returns the live filter, or null before one is set', () => {
        expect(store.getFilter()).toBeNull();
        store.setInitialFilter({ idProject: 1 } as IssuesFilter);
        store.setFilter({ title: 'login' } as IssuesFilterParams);
        expect(store.getFilter()).toMatchObject({ idProject: 1, title: 'login' });
        store.clear();
        expect(store.getFilter()).toBeNull();
    });

    it('toggleShowFilter flips the visibility flag', () => {
        expect(latest(store.showFilter$)).toBe(false);
        store.toggleShowFilter();
        expect(latest(store.showFilter$)).toBe(true);
    });

    it('actualFilterChange$ marks a filter change as refresh:false', () => {
        store.setFilter({ title: 'login' } as IssuesFilterParams);
        expect(latest(store.actualFilterChange$)?.refresh).toBe(false);
    });

    it('actualFilterChange$ marks refresh() as refresh:true (so paginated views keep their pages)', () => {
        store.setInitialFilter({ idProject: 1 } as IssuesFilter);
        store.refresh();
        const emission = latest(store.actualFilterChange$);
        expect(emission?.refresh).toBe(true);
        expect(emission?.filter).toMatchObject({ idProject: 1 });
    });
});
