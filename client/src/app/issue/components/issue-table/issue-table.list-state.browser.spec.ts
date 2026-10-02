import { TestBed } from '@angular/core/testing';
import { beforeEach, describe, expect, it } from 'vitest';
import { SavedView } from 'src/app/project/model/saved-view.model';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { IssueViewMode } from '../../constants/issue-view-modes.enum';
import { IssuesFilter } from '../filter/issue-filter.entity';
import { configureTableTestBed, TableMocks } from './table-testbed.helper';
import { IssueTableComponent } from './issue-table.component';

describe('IssueTableComponent list state (TestBed)', () => {
    let mocks: TableMocks;

    const idProject = 10;

    const remembered: IssuesFilter = {
        idProject,
        title: 'login',
        idsState: [2],
        orderColumn: 'title',
        orderDirection: 'asc'
    };

    async function mount() {
        await TestBed.compileComponents();
        const fixture = TestBed.createComponent(IssueTableComponent);
        fixture.detectChanges();
        await fixture.whenStable();
        return fixture;
    }

    function appliedFilter(): Record<string, unknown> {
        const calls = mocks.issueFilterStoreMock.setInitialFilter.mock.calls;
        return calls[calls.length - 1][0];
    }

    beforeEach(() => {
        localStorage.clear();
        mocks = configureTableTestBed();
    });

    it('shows the remembered filter instead of the defaults', async () => {
        mocks.listStateMock.restoreFilter.mockReturnValue(remembered);

        await mount();

        expect(appliedFilter()).toEqual(remembered);
    });

    it('still lets a staged saved view win over the remembered filter', async () => {
        mocks.listStateMock.restoreFilter.mockReturnValue(remembered);
        const view: SavedView = {
            idSavedView: 3,
            idProject,
            name: 'View',
            viewType: IssueViewMode.TABLE,
            config: { v: 1, idsState: [9] },
            isShared: false,
            createBy: 1,
            updateAt: '2026-08-01T00:00:00Z'
        };
        TestBed.inject(SavedViewStore).setPending(view, idProject);

        await mount();

        expect(appliedFilter()['idsState']).toEqual([9]);
    });

    it('drops the remembered filter and installs the defaults when the view is reset', async () => {
        mocks.listStateMock.restoreFilter.mockReturnValueOnce(remembered).mockReturnValue(null);
        await mount();

        TestBed.inject(SavedViewStore).sendFilterResetSignal();

        expect(mocks.listStateMock.forgetFilter).toHaveBeenCalledTimes(1);
        expect(appliedFilter()['stateUnset']).toBe(true);
    });

    it('asks the service for the rows that were loaded before leaving', async () => {
        mocks.listStateMock.restorePosition.mockReturnValue({
            loadedCount: 150,
            scrollTop: 400,
            scrollLeft: 0
        });

        await mount();

        expect(mocks.issueTableServiceMock.restoreLoadedCount).toHaveBeenCalledWith(150);
    });

    it('reports how many rows are loaded so the position can be kept on leaving', async () => {
        await mount();
        mocks.issueTableServiceMock.rows.set([
            { issue: { idIssuePublic: 1 } },
            { issue: { idIssuePublic: 2 } }
        ]);

        const readPosition = mocks.listStateMock.registerPosition.mock.calls[0][0];

        expect(readPosition()).toEqual({ loadedCount: 2, scrollTop: 0, scrollLeft: 0 });
    });

    it('stops reporting its position once destroyed', async () => {
        const fixture = await mount();
        const readPosition = mocks.listStateMock.registerPosition.mock.calls[0][0];

        fixture.destroy();

        expect(mocks.listStateMock.unregisterPosition).toHaveBeenCalledWith(readPosition);
    });
});
