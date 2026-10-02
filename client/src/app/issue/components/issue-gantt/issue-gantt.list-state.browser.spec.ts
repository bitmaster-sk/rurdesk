import { TestBed } from '@angular/core/testing';
import { beforeEach, describe, expect, it } from 'vitest';
import { IssuesFilter } from '../filter/issue-filter.entity';
import { configureGanttTestBed, GanttMocks } from './gantt-testbed.helper';
import { IssueGanttComponent } from './issue-gantt.component';

describe('IssueGanttComponent list state (TestBed)', () => {
    let mocks: GanttMocks;

    const remembered: IssuesFilter = {
        idProject: 1,
        title: 'release',
        orderColumn: 'scheduledAt',
        orderDirection: 'asc',
        scheduledAtFrom: new Date(2020, 0, 1),
        scheduledAtTo: new Date(2020, 1, 1)
    };

    async function mount() {
        await TestBed.compileComponents();
        const fixture = TestBed.createComponent(IssueGanttComponent);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        return fixture;
    }

    function appliedFilter(): IssuesFilter {
        const calls = mocks.issueFilterStoreMock.setInitialFilter.mock.calls;
        return calls[calls.length - 1][0];
    }

    beforeEach(() => {
        localStorage.clear();
        mocks = configureGanttTestBed();
    });

    it('shows the remembered filter around today, not the remembered window', async () => {
        mocks.listStateMock.restoreFilter.mockReturnValue(remembered);

        await mount();

        expect(appliedFilter().title).toBe('release');
        expect(appliedFilter().scheduledAtFrom!.getTime()).toBeGreaterThan(
            new Date(2020, 1, 1).getTime()
        );
    });

    it('asks for the backlog rows that were loaded before leaving', async () => {
        mocks.listStateMock.restorePosition.mockReturnValue({
            loadedCount: 90,
            scrollTop: 300,
            scrollLeft: 1200
        });

        await mount();

        expect(mocks.ganttServiceMock.restoreBacklogCount).toHaveBeenCalledWith(90);
    });

    it('reports the backlog count and timeline scroll so they can be kept on leaving', async () => {
        await mount();

        const readPosition = mocks.listStateMock.registerPosition.mock.calls[0][0];

        expect(readPosition()).toEqual(expect.objectContaining({ loadedCount: 0, scrollLeft: 0 }));
    });

    it('stops reporting its position once destroyed', async () => {
        const fixture = await mount();
        const readPosition = mocks.listStateMock.registerPosition.mock.calls[0][0];

        fixture.destroy();

        expect(mocks.listStateMock.unregisterPosition).toHaveBeenCalledWith(readPosition);
    });
});
