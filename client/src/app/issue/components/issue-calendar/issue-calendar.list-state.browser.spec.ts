import { TestBed } from '@angular/core/testing';
import { beforeEach, describe, expect, it } from 'vitest';
import { IssuesFilter } from '../filter/issue-filter.entity';
import { CalendarMocks, configureCalendarTestBed } from './calendar-testbed.helper';
import { IssueCalendarComponent } from './issue-calendar.component';

describe('IssueCalendarComponent list state (TestBed)', () => {
    let mocks: CalendarMocks;

    const remembered: IssuesFilter = {
        idProject: 1,
        title: 'release',
        orderColumn: 'scheduledAt',
        orderDirection: 'desc',
        scheduledAtFrom: new Date(2025, 0, 1),
        scheduledAtTo: new Date(2025, 1, 1)
    };

    async function mount() {
        await TestBed.compileComponents();
        const fixture = TestBed.createComponent(IssueCalendarComponent);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        return fixture;
    }

    beforeEach(() => {
        localStorage.clear();
        mocks = configureCalendarTestBed();
    });

    it('shows the remembered filter for the dates on screen, not the remembered dates', async () => {
        mocks.listStateMock.restoreFilter.mockReturnValue(remembered);

        await mount();

        expect(mocks.issueFilterStoreMock.setInitialFilter).toHaveBeenCalledWith({
            ...remembered,
            scheduledAtFrom: new Date(2026, 8, 1),
            scheduledAtTo: new Date(2026, 9, 1)
        });
    });

    it('loads the dates on screen when nothing is remembered', async () => {
        await mount();

        expect(mocks.issueFilterStoreMock.setInitialFilter).toHaveBeenCalledWith(
            expect.objectContaining({
                scheduledAtFrom: new Date(2026, 8, 1),
                scheduledAtTo: new Date(2026, 9, 1)
            })
        );
    });

    it('opens on the date and granularity the user left on the back button', async () => {
        mocks.listStateMock.restorePosition.mockReturnValue({
            loadedCount: 0,
            scrollTop: 0,
            scrollLeft: 0,
            date: new Date(2026, 2, 10),
            calendarView: 'timeGridWeek'
        });

        const fixture = await mount();
        const comp: IssueCalendarComponent = fixture.componentInstance;

        expect(comp.defaultCalendarOps.initialDate).toEqual(new Date(2026, 2, 10));
        expect(comp.defaultCalendarOps.initialView).toBe('timeGridWeek');
        expect(comp.currentView).toBe('timeGridWeek');
    });

    it('reports the shown date and granularity so they can be kept on leaving', async () => {
        const fixture = await mount();
        fixture.componentInstance.onViewModeChange('timeGridDay');

        const readPosition = mocks.listStateMock.registerPosition.mock.calls[0][0];

        expect(readPosition()).toEqual(
            expect.objectContaining({ date: new Date(2026, 8, 15), calendarView: 'timeGridDay' })
        );
    });

    it('stops reporting its position once destroyed', async () => {
        const fixture = await mount();
        const readPosition = mocks.listStateMock.registerPosition.mock.calls[0][0];

        fixture.destroy();

        expect(mocks.listStateMock.unregisterPosition).toHaveBeenCalledWith(readPosition);
    });
});
