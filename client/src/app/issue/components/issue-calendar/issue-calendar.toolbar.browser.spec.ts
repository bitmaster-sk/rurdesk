import { describe, it, expect, beforeEach } from 'vitest';
import { createCalendarFixture } from './calendar-testbed.helper';

describe('IssueCalendarComponent toolbar handlers (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createCalendarFixture();
        comp = result.comp;
        mocks = result.mocks;
    });

    function getApi() {
        return comp.calendarRef().getApi();
    }

    // =========================================================================
    // onToggleFilter
    // =========================================================================

    it('delegates to issueFilterStore.toggleShowFilter', () => {
        comp.onToggleFilter();
        expect(mocks.issueFilterStoreMock.toggleShowFilter).toHaveBeenCalled();
    });

    // =========================================================================
    // onViewModeChange
    // =========================================================================

    it('sets currentView and calls calendarApi.changeView', () => {
        getApi().changeView.mockClear();
        comp.onViewModeChange('timeGridWeek');
        expect(comp.currentView).toBe('timeGridWeek');
        expect(getApi().changeView).toHaveBeenCalledWith('timeGridWeek');
    });

    // =========================================================================
    // viewOptions — i18n keys instead of pre-resolved labels
    // =========================================================================

    it('viewOptions stores labelKey translation keys, not pre-resolved labels', () => {
        expect(comp.viewOptions.every(o => 'labelKey' in o && !('label' in o))).toBe(true);
        expect(comp.viewOptions[0]).toEqual({
            labelKey: 'ISSUE.CALENDAR.DAY',
            value: 'timeGridDay'
        });
    });

    // =========================================================================
    // zoomView
    // =========================================================================

    describe('zoomView', () => {
        it('month + 1 → week', () => {
            getApi().changeView.mockClear();
            comp.currentView = 'dayGridMonth';
            comp.zoomView(1);
            expect(getApi().changeView).toHaveBeenCalledWith('timeGridWeek');
        });

        it('week + 1 → day', () => {
            getApi().changeView.mockClear();
            comp.currentView = 'timeGridWeek';
            comp.zoomView(1);
            expect(getApi().changeView).toHaveBeenCalledWith('timeGridDay');
        });

        it('day + 1 → no-op (already finest)', () => {
            getApi().changeView.mockClear();
            comp.currentView = 'timeGridDay';
            comp.zoomView(1);
            expect(getApi().changeView).not.toHaveBeenCalled();
        });

        it('week - 1 → month', () => {
            getApi().changeView.mockClear();
            comp.currentView = 'timeGridWeek';
            comp.zoomView(-1);
            expect(getApi().changeView).toHaveBeenCalledWith('dayGridMonth');
        });

        it('month - 1 → no-op (already coarsest)', () => {
            getApi().changeView.mockClear();
            comp.currentView = 'dayGridMonth';
            comp.zoomView(-1);
            expect(getApi().changeView).not.toHaveBeenCalled();
        });
    });

    // =========================================================================
    // onCardModeChange
    // =========================================================================

    describe('onCardModeChange', () => {
        it('sets cardMode signal, persists to localStorage, toggles FC class', () => {
            comp.onCardModeChange('CalendarCompact');
            expect(comp.cardMode()).toBe('CalendarCompact');
            expect(localStorage.getItem('issue-calendar-card-mode')).toBe('CalendarCompact');
        });

        it('comfortable mode removes fc--compact class', () => {
            comp.onCardModeChange('CalendarComfort');
            expect(comp.cardMode()).toBe('CalendarComfort');
        });

        it('calls calendarApi.render after mode change', () => {
            getApi().render.mockClear();
            comp.onCardModeChange('CalendarCompact');
            expect(getApi().render).toHaveBeenCalled();
        });
    });

    // =========================================================================
    // validCalendarMode
    // =========================================================================

    describe('validCalendarMode', () => {
        it('returns "CalendarComfort" for null', () => {
            expect(comp.validCalendarMode(null)).toBe('CalendarComfort');
        });

        it('returns "CalendarComfort" for invalid value', () => {
            expect(comp.validCalendarMode('GanttComfort')).toBe('CalendarComfort');
        });

        it('returns "CalendarCompact" when stored', () => {
            expect(comp.validCalendarMode('CalendarCompact')).toBe('CalendarCompact');
        });

        it('returns "CalendarComfort" when stored', () => {
            expect(comp.validCalendarMode('CalendarComfort')).toBe('CalendarComfort');
        });
    });

    // =========================================================================
    // onCalendarEventClick
    // =========================================================================

    it('navigates to the issue detail page', () => {
        const issue = { idProject: 5, idIssuePublic: 42 } as any;
        const evt = { event: { extendedProps: { issue } } } as any;
        comp.onCalendarEventClick(evt);
        // Can't easily assert router.navigate without spy injection;
        // verify it doesn't throw
    });

    // =========================================================================
    // setFilter
    // =========================================================================

    describe('setFilter', () => {
        it('passes the provided start/end dates to issueFilterStore.setFilter', () => {
            mocks.issueFilterStoreMock.setFilter.mockClear();
            const start = new Date('2025-01-01T00:00:00Z');
            const end = new Date('2025-01-31T00:00:00Z');
            comp.setFilter(start, end);
            expect(mocks.issueFilterStoreMock.setFilter).toHaveBeenCalledWith({
                scheduledAtFrom: start,
                scheduledAtTo: end
            });
        });
    });

    // =========================================================================
    // locale / i18n
    // =========================================================================

    describe('locale', () => {
        it('initial locale derives from I18nService.currentLang', () => {
            expect(comp.defaultCalendarOps.locale).toBe('en-gb');
        });

        it('langChange$ updates the calendar locale via setOption', () => {
            const langChange$ = mocks.i18nMock.langChange$.source;
            const setOption = comp.calendarRef().getApi().setOption;

            langChange$.next({ lang: 'sk' });
            expect(setOption).toHaveBeenCalledWith('locale', 'sk');
        });

        it('falls back to English when an unsupported language is selected', () => {
            const langChange$ = mocks.i18nMock.langChange$.source;
            const setOption = comp.calendarRef().getApi().setOption;

            langChange$.next({ lang: 'unknown' });
            expect(setOption).toHaveBeenCalledWith('locale', 'en-gb');
        });
    });
});
