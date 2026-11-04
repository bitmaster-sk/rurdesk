import { describe, expect, it } from 'vitest';
import { Issue } from '../model/issue.model';
import { IssuesFilter } from '../components/filter/issue-filter.entity';
import { BULK_SELECTION_LIMIT, IssueSelectionStore } from './issue-selection.store';

const issue = (idIssuePublic: number, extra: Partial<Issue> = {}): Issue => ({
    idIssue: idIssuePublic * 10,
    idIssuePublic,
    idProject: 1,
    idState: 1,
    idSeverity: null,
    idIssueType: null,
    title: `Task ${idIssuePublic}`,
    description: '',
    tracked: 0,
    ...extra
});

const rows = (count: number): Issue[] => Array.from({ length: count }, (_, i) => issue(i + 1));

const ids = (store: IssueSelectionStore): number[] =>
    store
        .selected()
        .map(i => i.idIssuePublic)
        .sort((a, b) => a - b);

const filterOf = (extra: Partial<IssuesFilter>): IssuesFilter => ({
    idProject: 1,
    orderColumn: 'idIssue',
    orderDirection: 'desc',
    ...extra
});

describe('IssueSelectionStore', () => {
    it('toggle adds a task and removes it on the second call', () => {
        const store = new IssueSelectionStore();
        const task = issue(7);

        store.toggle(task);
        expect(store.isSelected(7)).toBe(true);
        expect(store.count()).toBe(1);

        store.toggle(task);
        expect(store.isSelected(7)).toBe(false);
        expect(store.count()).toBe(0);
    });

    it('selectRange adds every task between the last toggled one and the target', () => {
        const store = new IssueSelectionStore();
        const list = rows(6);

        store.toggle(list[1]);
        store.selectRange(list[4], list);

        expect(ids(store)).toEqual([2, 3, 4, 5]);
    });

    it('selectRange works upwards and keeps tasks selected outside the range', () => {
        const store = new IssueSelectionStore();
        const list = rows(6);

        store.toggle(list[5]);
        store.toggle(list[3]);
        store.selectRange(list[1], list);

        expect(ids(store)).toEqual([2, 3, 4, 6]);
    });

    it('selectRange without an anchor in the list just toggles the target', () => {
        const store = new IssueSelectionStore();
        const list = rows(4);

        store.selectRange(list[2], list);

        expect(ids(store)).toEqual([3]);
    });

    it('extendTo grows and shrinks the range from the anchor like shift+arrow', () => {
        const store = new IssueSelectionStore();
        const list = rows(6);

        store.extendTo(list[2], list, 2);
        expect(ids(store)).toEqual([2, 3]);

        store.extendTo(list[3], list, 3);
        expect(ids(store)).toEqual([2, 3, 4]);

        store.extendTo(list[2], list, 4);
        expect(ids(store)).toEqual([2, 3]);
    });

    it('shrinking a range keeps tasks that were picked before it', () => {
        const store = new IssueSelectionStore();
        const list = rows(5);
        store.toggle(list[1]);
        store.toggle(list[3]);

        store.extendTo(list[2], list, 4);
        store.extendTo(list[1], list, 3);
        store.extendTo(list[2], list, 2);

        expect(ids(store)).toEqual([2, 3, 4]);
    });

    it('extendTo starts from the last toggled task', () => {
        const store = new IssueSelectionStore();
        const list = rows(6);
        store.toggle(list[4]);

        store.extendTo(list[2], list, 2);

        expect(ids(store)).toEqual([3, 4, 5]);
    });

    it('toggleAll selects all given tasks, and clears them when all are selected', () => {
        const store = new IssueSelectionStore();
        const list = rows(3);

        store.toggle(list[0]);
        store.toggleAll(list);
        expect(ids(store)).toEqual([1, 2, 3]);

        store.toggleAll(list);
        expect(ids(store)).toEqual([]);
    });

    it(`stops at ${BULK_SELECTION_LIMIT} tasks and reports the limit`, () => {
        const store = new IssueSelectionStore();
        const list = rows(BULK_SELECTION_LIMIT + 5);

        const result = store.toggleAll(list);

        expect(result).toBe('limit');
        expect(store.count()).toBe(BULK_SELECTION_LIMIT);
        expect(store.isLimitReached()).toBe(true);
        expect(store.toggle(list[BULK_SELECTION_LIMIT + 1])).toBe('limit');
        expect(store.count()).toBe(BULK_SELECTION_LIMIT);
    });

    it('refresh swaps in fresh task data only for selected tasks', () => {
        const store = new IssueSelectionStore();
        store.toggle(issue(1, { idsTag: [] }));

        store.refresh([issue(1, { idsTag: [5] }), issue(2)]);

        expect(store.selected()).toHaveLength(1);
        expect(store.selected()[0].idsTag).toEqual([5]);
    });

    it('remove drops a task and no longer uses it as the range anchor', () => {
        const store = new IssueSelectionStore();
        const list = rows(5);
        store.toggle(list[1]);
        store.toggle(list[3]);

        store.remove(4);
        store.selectRange(list[4], list);

        expect(ids(store)).toEqual([2, 5]);
    });

    it('clearOnScopeChange keeps the selection for a sort change and clears it for a filter change', () => {
        const store = new IssueSelectionStore();
        store.clearOnScopeChange(filterOf({ idsState: [1] }));
        store.toggle(issue(1));

        store.clearOnScopeChange(filterOf({ idsState: [1], orderColumn: 'title' }));
        expect(store.count()).toBe(1);

        store.clearOnScopeChange(filterOf({ idsState: [1, 2] }));
        expect(store.count()).toBe(0);
    });
});
