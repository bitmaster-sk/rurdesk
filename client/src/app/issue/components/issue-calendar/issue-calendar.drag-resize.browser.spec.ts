import { describe, it, expect, vi, beforeEach } from 'vitest';
import { add } from 'date-fns';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { createCalendarFixture, mockSub } from './calendar-testbed.helper';

function makeIssue(over: Partial<any> = {}) {
    return {
        idIssue: 1,
        idIssuePublic: 10,
        idProject: 5,
        title: 'Test Issue',
        scheduledAt: new Date('2025-01-15T09:00:00Z'),
        estimated: 3600,
        ...over
    };
}

function makeDropArg(over: Partial<any> = {}) {
    return {
        event: {
            extendedProps: { issue: makeIssue() },
            allDay: false,
            start: new Date('2025-01-16T09:00:00Z'),
            ...over.event
        },
        oldEvent: {
            allDay: false,
            ...over.oldEvent
        },
        delta: { years: 0, months: 0, days: 1, milliseconds: 0, ...over.delta },
        revert: over.revert ?? vi.fn()
    };
}

function makeResizeArg(over: Partial<any> = {}) {
    return {
        event: {
            extendedProps: { issue: makeIssue({ estimated: 3600 }) },
            ...over.event
        },
        startDelta: { years: 0, months: 0, days: 0, milliseconds: 0, ...over.startDelta },
        endDelta: { years: 0, months: 0, days: 1, milliseconds: 0, ...over.endDelta },
        revert: over.revert ?? vi.fn()
    };
}

describe('IssueCalendarComponent onCalendarEventDrop (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createCalendarFixture();
        comp = result.comp;
        mocks = result.mocks;
    });

    it('timed → timed: shifts scheduledAt by delta, calls updateIssue', () => {
        const sub = mockSub();
        mocks.sIssueMock.updateIssue.mockReturnValue(sub);
        const issue = makeIssue({ scheduledAt: new Date('2025-01-15T09:00:00Z'), estimated: 3600 });

        comp.onCalendarEventDrop(
            makeDropArg({
                event: {
                    extendedProps: { issue },
                    allDay: false,
                    start: new Date('2025-01-16T09:00:00Z')
                },
                oldEvent: { allDay: false },
                delta: { years: 0, months: 0, days: 1, milliseconds: 0 }
            })
        );

        const passedIssue = mocks.sIssueMock.updateIssue.mock.calls[0][0];
        expect(passedIssue.scheduledAt).toEqual(
            add(new Date('2025-01-15T09:00:00Z'), { days: 1, months: 0, years: 0, seconds: 0 })
        );
    });

    it('timed → timed: shifts by milliseconds too (seconds truncated)', () => {
        const issue = makeIssue({ scheduledAt: new Date('2025-01-15T09:00:00Z'), estimated: 3600 });

        comp.onCalendarEventDrop(
            makeDropArg({
                event: {
                    extendedProps: { issue },
                    allDay: false,
                    start: new Date('2025-01-15T09:30:00Z')
                },
                oldEvent: { allDay: false },
                delta: { years: 0, months: 0, days: 0, milliseconds: 1800000 }
            })
        );

        const passedIssue = mocks.sIssueMock.updateIssue.mock.calls[0][0];
        expect(passedIssue.scheduledAt).toEqual(
            add(new Date('2025-01-15T09:00:00Z'), { seconds: 1800 })
        );
    });

    it('allDay → timed: sets scheduledAt to event.start, estimated to 1h', () => {
        const issue = makeIssue({ scheduledAt: new Date('2025-01-15T00:00:00Z'), estimated: null });

        comp.onCalendarEventDrop(
            makeDropArg({
                event: {
                    extendedProps: { issue },
                    allDay: false,
                    start: new Date('2025-01-16T10:00:00Z')
                },
                oldEvent: { allDay: true },
                delta: { years: 0, months: 0, days: 1, milliseconds: 0 }
            })
        );

        const passedIssue = mocks.sIssueMock.updateIssue.mock.calls[0][0];
        expect(passedIssue.scheduledAt).toEqual(new Date('2025-01-16T10:00:00Z'));
        expect(passedIssue.estimated).toBe(3600);
    });

    it('timed → allDay: sets estimated to null', () => {
        const issue = makeIssue({ scheduledAt: new Date('2025-01-15T09:00:00Z'), estimated: 3600 });

        comp.onCalendarEventDrop(
            makeDropArg({
                event: { extendedProps: { issue }, allDay: true },
                oldEvent: { allDay: false },
                delta: { years: 0, months: 0, days: 1, milliseconds: 0 }
            })
        );

        const passedIssue = mocks.sIssueMock.updateIssue.mock.calls[0][0];
        expect(passedIssue.estimated).toBeNull();
    });

    it('on API error: calls revert and shows a toast', () => {
        const sub = mockSub();
        mocks.sIssueMock.updateIssue.mockReturnValue(sub);
        const revertSpy = vi.fn();
        const issue = makeIssue({ scheduledAt: new Date('2025-01-15T09:00:00Z'), estimated: 3600 });

        comp.onCalendarEventDrop(
            makeDropArg({
                event: { extendedProps: { issue }, allDay: false },
                oldEvent: { allDay: false },
                delta: { years: 0, months: 0, days: 1, milliseconds: 0 },
                revert: revertSpy
            })
        );

        sub.handlers.error?.(new Error('fail'));
        expect(revertSpy).toHaveBeenCalled();
        expect(mocks.toastMock.showError).toHaveBeenCalledWith('ISSUE.CALENDAR_UPDATE_FAILED');
    });

    it('cloneDeep: original issue is not mutated', () => {
        const issue = makeIssue({ scheduledAt: new Date('2025-01-15T09:00:00Z'), estimated: 3600 });
        const originalScheduledAt = issue.scheduledAt;

        comp.onCalendarEventDrop(
            makeDropArg({
                event: { extendedProps: { issue }, allDay: false },
                oldEvent: { allDay: false },
                delta: { years: 0, months: 0, days: 2, milliseconds: 0 }
            })
        );

        expect(issue.scheduledAt).toEqual(originalScheduledAt);
        expect(mocks.sIssueMock.updateIssue.mock.calls[0][0]).not.toBe(issue);
    });

    it('without scheduledAt: reverts without updating', () => {
        const revertSpy = vi.fn();
        const issue = makeIssue({ scheduledAt: null, estimated: 3600 });

        vi.spyOn(console, 'warn').mockImplementation(() => {});
        comp.onCalendarEventDrop(
            makeDropArg({
                event: { extendedProps: { issue }, allDay: false },
                oldEvent: { allDay: false },
                delta: { years: 0, months: 0, days: 1, milliseconds: 0 },
                revert: revertSpy
            })
        );

        expect(revertSpy).toHaveBeenCalled();
        expect(mocks.sIssueMock.updateIssue).not.toHaveBeenCalled();
    });
});

describe('IssueCalendarComponent onCalendarEventResize (TestBed)', () => {
    let comp: any;
    let mocks: any;

    beforeEach(async () => {
        localStorage.clear();
        const result = await createCalendarFixture();
        comp = result.comp;
        mocks = result.mocks;
    });

    it('with valid end delta: adds deltaSeconds to estimated, calls updateIssue', () => {
        const sub = mockSub();
        mocks.sIssueMock.updateIssue.mockReturnValue(sub);
        const issue = makeIssue({ estimated: 3600 });

        comp.onCalendarEventResize(
            makeResizeArg({
                event: { extendedProps: { issue } },
                startDelta: { years: 0, months: 0, days: 0, milliseconds: 0 },
                endDelta: { years: 0, months: 0, days: 1, milliseconds: 0 }
            })
        );

        const expectedDelta = DurationConverter.durationToSeconds({ days: 1, seconds: 0 });
        const passedIssue = mocks.sIssueMock.updateIssue.mock.calls[0][0];
        expect(passedIssue.estimated).toBe(3600 + expectedDelta);
        expect(issue.estimated).toBe(3600);
    });

    it('with milliseconds in endDelta: truncates to seconds', () => {
        const issue = makeIssue({ estimated: 3600 });

        comp.onCalendarEventResize(
            makeResizeArg({
                event: { extendedProps: { issue } },
                startDelta: { years: 0, months: 0, days: 0, milliseconds: 0 },
                endDelta: { years: 0, months: 0, days: 0, milliseconds: 5400000 }
            })
        );

        const passedIssue = mocks.sIssueMock.updateIssue.mock.calls[0][0];
        expect(passedIssue.estimated).toBe(3600 + 5400);
    });

    it('with non-zero startDelta: reverts without updating', () => {
        const revertSpy = vi.fn();
        const issue = makeIssue({ estimated: 3600 });

        comp.onCalendarEventResize(
            makeResizeArg({
                event: { extendedProps: { issue } },
                startDelta: { years: 0, months: 0, days: 1, milliseconds: 0 },
                endDelta: { years: 0, months: 0, days: 1, milliseconds: 0 },
                revert: revertSpy
            })
        );

        expect(revertSpy).toHaveBeenCalled();
        expect(mocks.sIssueMock.updateIssue).not.toHaveBeenCalled();
    });

    it('with startDelta milliseconds only (non-zero): reverts', () => {
        const revertSpy = vi.fn();
        const issue = makeIssue({ estimated: 3600 });

        comp.onCalendarEventResize(
            makeResizeArg({
                event: { extendedProps: { issue } },
                startDelta: { years: 0, months: 0, days: 0, milliseconds: 1000 },
                endDelta: { years: 0, months: 0, days: 0, milliseconds: 0 },
                revert: revertSpy
            })
        );

        expect(revertSpy).toHaveBeenCalled();
    });

    it('with all-zero startDelta and negative endDelta: reduces estimated', () => {
        const issue = makeIssue({ estimated: 7200 });

        comp.onCalendarEventResize(
            makeResizeArg({
                event: { extendedProps: { issue } },
                startDelta: { years: 0, months: 0, days: 0, milliseconds: 0 },
                endDelta: { years: 0, months: 0, days: -1, milliseconds: 0 }
            })
        );

        const expectedDelta = DurationConverter.durationToSeconds({ days: -1, seconds: 0 });
        const passedIssue = mocks.sIssueMock.updateIssue.mock.calls[0][0];
        expect(passedIssue.estimated).toBe(7200 + expectedDelta);
    });

    it('on API error: calls revert', () => {
        const sub = mockSub();
        mocks.sIssueMock.updateIssue.mockReturnValue(sub);
        const revertSpy = vi.fn();
        const issue = makeIssue({ estimated: 3600 });

        vi.spyOn(console, 'error').mockImplementation(() => {});
        comp.onCalendarEventResize(
            makeResizeArg({
                event: { extendedProps: { issue } },
                startDelta: { years: 0, months: 0, days: 0, milliseconds: 0 },
                endDelta: { years: 0, months: 0, days: 1, milliseconds: 0 },
                revert: revertSpy
            })
        );

        expect(() => sub.handlers.error?.(new Error('fail'))).toThrow('fail');
        expect(revertSpy).toHaveBeenCalled();
    });

    it('cloneDeep: original issue is not mutated', () => {
        const issue = makeIssue({ estimated: 3600 });
        const originalEstimated = issue.estimated;

        comp.onCalendarEventResize(
            makeResizeArg({
                event: { extendedProps: { issue } },
                startDelta: { years: 0, months: 0, days: 0, milliseconds: 0 },
                endDelta: { years: 0, months: 0, days: 1, milliseconds: 0 }
            })
        );

        expect(issue.estimated).toBe(originalEstimated);
        expect(mocks.sIssueMock.updateIssue.mock.calls[0][0]).not.toBe(issue);
    });
});
