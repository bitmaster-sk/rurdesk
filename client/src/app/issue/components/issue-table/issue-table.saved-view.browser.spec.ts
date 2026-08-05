import { TestBed } from '@angular/core/testing';
import { beforeEach, describe, expect, it } from 'vitest';
import { SavedView } from 'src/app/project/model/saved-view.model';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { configureTableTestBed, TableMocks } from './table-testbed.helper';
import { IssueTableComponent } from './issue-table.component';

describe('IssueTableComponent saved view handoff (TestBed)', () => {
    let mocks: TableMocks;
    let store: SavedViewStore;

    // The harness' ProjectStore emits idProject 10.
    const idProject = 10;

    function view(config: SavedView['config']): SavedView {
        return { idSavedView: 3, idProject, viewType: 'table', config } as SavedView;
    }

    async function mount(): Promise<void> {
        await TestBed.compileComponents();
        const fixture = TestBed.createComponent(IssueTableComponent);
        fixture.detectChanges();
        await fixture.whenStable();
    }

    function appliedFilter(): Record<string, unknown> {
        const calls = mocks.issueFilterStoreMock.setInitialFilter.mock.calls;
        return calls[calls.length - 1][0];
    }

    beforeEach(() => {
        localStorage.clear();
        mocks = configureTableTestBed();
        store = TestBed.inject(SavedViewStore);
    });

    it('applies a staged view instead of the hardcoded defaults', async () => {
        store.setPending(view({ v: 1, idsState: [9], orderColumn: 'title' }), idProject);

        await mount();

        expect(mocks.issueFilterStoreMock.setInitialFilter).toHaveBeenCalledTimes(1);
        expect(appliedFilter()).toEqual({
            idProject,
            idsState: [9],
            orderColumn: 'title',
            orderDirection: 'desc'
        });
    });

    it('does not merge the defaults into the applied view', async () => {
        store.setPending(view({ v: 1, idsState: [9] }), idProject);

        await mount();

        expect(appliedFilter()['stateUnset']).toBeUndefined();
        expect(appliedFilter()['severityUnset']).toBeUndefined();
    });

    it('installs the defaults when nothing is staged', async () => {
        await mount();

        expect(appliedFilter()['stateUnset']).toBe(true);
        expect(appliedFilter()['severityUnset']).toBe(true);
    });

    // A later plain navigation (sidebar click) must not re-apply the view.
    it('consumes the staged view exactly once', async () => {
        store.setPending(view({ v: 1, idsState: [9] }), idProject);
        await mount();

        expect(store.consumePending(idProject)).toBeNull();
    });

    it('reinstalls its defaults when the applied view is cleared', async () => {
        store.setPending(view({ v: 1, idsState: [9] }), idProject);
        await mount();
        expect(appliedFilter()['idsState']).toEqual([9]);

        store.sendFilterResetSignal();

        expect(appliedFilter()['stateUnset']).toBe(true);
        expect(mocks.issueFilterStoreMock.setInitialFilter).toHaveBeenCalledTimes(2);
    });

    it('ignores a view staged for a different project', async () => {
        store.setPending(view({ v: 1, idsState: [9] }), idProject + 1);

        await mount();

        expect(appliedFilter()['stateUnset']).toBe(true);
    });
});
