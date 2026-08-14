import { describe, expect, it } from 'vitest';
import { SprintState } from '../constants/sprint-state.enum';
import { SprintUnit } from '../constants/sprint-unit.enum';
import { SprintBurndown, SprintBurndownDay } from '../model/sprint-burndown.model';
import { SprintVelocity } from '../model/sprint-velocity.model';
import { SprintAnalyticsConverter } from './sprint-analytics.converter';

function day(
    date: string,
    scope: number | null,
    done: number | null,
    snapshot: boolean
): SprintBurndownDay {
    return {
        day: `${date}T00:00:00Z`,
        totalPoints: scope,
        donePoints: done,
        remainingPoints: scope === null || done === null ? null : scope - done,
        totalIssues: scope === null ? null : scope * 2,
        doneIssues: done === null ? null : done * 2,
        remainingIssues: scope === null || done === null ? null : (scope - done) * 2,
        snapshot
    };
}

function burndown(
    days: SprintBurndownDay[],
    startAt = '2026-05-01',
    endAt = '2026-05-05'
): SprintBurndown {
    return {
        idSprint: 1,
        startAt: `${startAt}T00:00:00Z`,
        endAt: `${endAt}T00:00:00Z`,
        state: SprintState.Planned,
        days
    };
}

function velocity(
    name: string,
    donePoints: number,
    doneIssues: number,
    planned?: number
): SprintVelocity {
    return {
        idSprint: name.length,
        name,
        endAt: '2026-05-01T00:00:00Z',
        donePoints,
        doneIssues,
        plannedPoints: planned,
        plannedIssues: planned === undefined ? undefined : planned * 2,
        frozen: planned !== undefined
    };
}

describe('SprintAnalyticsConverter.toBurndownSeries', () => {
    it('maps the payload to parallel arrays in the chosen unit', () => {
        const series = SprintAnalyticsConverter.toBurndownSeries(
            burndown([day('2026-05-01', 20, 0, true), day('2026-05-02', 20, 8, false)]),
            SprintUnit.Points
        );

        expect(series.labels).toEqual(['2026-05-01T00:00:00Z', '2026-05-02T00:00:00Z']);
        expect(series.remaining).toEqual([20, 12]);
        expect(series.done).toEqual([0, 8]);
        expect(series.scope).toEqual([20, 20]);
        expect(series.isReal).toEqual([true, false]);
    });

    it('reads the tasks unit from the issue columns', () => {
        const series = SprintAnalyticsConverter.toBurndownSeries(
            burndown([day('2026-05-01', 20, 5, true)]),
            SprintUnit.Issues
        );

        expect(series.scope).toEqual([40]);
        expect(series.remaining).toEqual([30]);
    });

    it('survives an empty payload', () => {
        const series = SprintAnalyticsConverter.toBurndownSeries(null, SprintUnit.Points);

        expect(series.labels).toEqual([]);
        expect(series.remaining).toEqual([]);
    });
});

describe('SprintAnalyticsConverter.toIdealLine', () => {
    it('runs from the baseline scope to zero on the end day', () => {
        const ideal = SprintAnalyticsConverter.toIdealLine(
            burndown([
                day('2026-05-01', 20, 0, true),
                day('2026-05-02', 24, 4, true),
                day('2026-05-03', 24, 10, true),
                day('2026-05-04', 24, 16, true),
                day('2026-05-05', 24, 24, true)
            ]),
            SprintUnit.Points
        );

        expect(ideal).toEqual([20, 15, 10, 5, 0]);
    });

    it('stays null beyond the planned end when the cycle overruns', () => {
        const ideal = SprintAnalyticsConverter.toIdealLine(
            burndown([
                day('2026-05-01', 10, 0, true),
                day('2026-05-02', 10, 2, true),
                day('2026-05-03', 10, 4, true),
                day('2026-05-04', 10, 6, true),
                day('2026-05-05', 10, 8, true),
                day('2026-05-06', 10, 9, true)
            ]),
            SprintUnit.Points
        );

        expect(ideal[4]).toBe(0);
        expect(ideal[5]).toBeNull();
    });

    it('degenerates to a single point for a one-day window', () => {
        const ideal = SprintAnalyticsConverter.toIdealLine(
            burndown([day('2026-05-01', 8, 0, true)], '2026-05-01', '2026-05-01'),
            SprintUnit.Points
        );

        expect(ideal).toEqual([8]);
    });

    it('returns an empty line for an empty payload', () => {
        expect(SprintAnalyticsConverter.toIdealLine(null, SprintUnit.Points)).toEqual([]);
    });
});

describe('SprintAnalyticsConverter.toScopeAdded', () => {
    it('reports the largest increase with its date and 1-based day index', () => {
        const change = SprintAnalyticsConverter.toScopeAdded(
            burndown([
                day('2026-05-01', 20, 0, true),
                day('2026-05-02', 21, 2, true),
                day('2026-05-03', 24, 6, true),
                day('2026-05-04', 24, 9, false),
                day('2026-05-05', 24, 12, true)
            ]),
            SprintUnit.Points
        );

        expect(change.delta).toBe(4);
        expect(change.day).toBe('2026-05-03T00:00:00Z');
        expect(change.dayIndex).toBe(3);
        expect(change.daysTotal).toBe(4);
    });

    it('reports a removal when scope only shrinks', () => {
        const change = SprintAnalyticsConverter.toScopeAdded(
            burndown([
                day('2026-05-01', 20, 0, true),
                day('2026-05-02', 17, 2, true),
                day('2026-05-03', 16, 6, true)
            ]),
            SprintUnit.Points
        );

        expect(change.delta).toBe(-4);
        expect(change.day).toBe('2026-05-02T00:00:00Z');
        expect(change.dayIndex).toBe(2);
    });

    it('names no day when a round trip nets out to zero', () => {
        const change = SprintAnalyticsConverter.toScopeAdded(
            burndown([
                day('2026-05-01', 20, 0, true),
                day('2026-05-02', 23, 2, true),
                day('2026-05-03', 20, 6, true)
            ]),
            SprintUnit.Points
        );

        expect(change.delta).toBe(0);
        expect(change.day).toBeNull();
        expect(change.dayIndex).toBeNull();
    });

    it('counts a folded pre-start baseline as the first point of the sequence', () => {
        const change = SprintAnalyticsConverter.toScopeAdded(
            burndown([
                day('2026-05-01', 12, 0, false),
                day('2026-05-02', 18, 3, true),
                day('2026-05-03', 18, 7, false)
            ]),
            SprintUnit.Points
        );

        expect(change.delta).toBe(6);
        expect(change.day).toBe('2026-05-02T00:00:00Z');
        expect(change.dayIndex).toBe(2);
    });

    it('has no day to name from a single recorded point', () => {
        const change = SprintAnalyticsConverter.toScopeAdded(
            burndown([day('2026-05-01', 20, 0, true), day('2026-05-02', 20, 3, false)]),
            SprintUnit.Points
        );

        expect(change.delta).toBe(0);
        expect(change.day).toBeNull();
    });

    it('keeps the day index counting past the planned end', () => {
        const change = SprintAnalyticsConverter.toScopeAdded(
            burndown([
                day('2026-05-01', 20, 0, true),
                day('2026-05-05', 24, 6, true),
                day('2026-05-06', 24, 8, true)
            ]),
            SprintUnit.Points
        );

        expect(change.daysTotal).toBe(4);
        expect(change.dayIndex).toBe(5);
        expect(change.day).toBe('2026-05-05T00:00:00Z');
    });

    it('survives an empty payload', () => {
        const change = SprintAnalyticsConverter.toScopeAdded(null, SprintUnit.Points);

        expect(change.delta).toBe(0);
        expect(change.daysTotal).toBe(0);
    });
});

describe('SprintAnalyticsConverter.toVelocitySeries', () => {
    it('leaves a hole where a cycle has no frozen plan', () => {
        const series = SprintAnalyticsConverter.toVelocitySeries(
            [velocity('Old', 8, 4), velocity('New', 11, 5, 13)],
            SprintUnit.Points
        );

        expect(series.labels).toEqual(['Old', 'New']);
        expect(series.done).toEqual([8, 11]);
        expect(series.planned).toEqual([null, 13]);
    });

    it('reads the tasks unit', () => {
        const series = SprintAnalyticsConverter.toVelocitySeries(
            [velocity('New', 11, 5, 13)],
            SprintUnit.Issues
        );

        expect(series.done).toEqual([5]);
        expect(series.planned).toEqual([26]);
    });
});

describe('SprintAnalyticsConverter.toVelocityWindows', () => {
    it('splits the payload into a newer and an older half', () => {
        const entries = [1, 2, 3, 4, 5, 6].map(n => velocity(`S${n}`, n * 2, n));

        const windows = SprintAnalyticsConverter.toVelocityWindows(entries, SprintUnit.Points);

        expect(windows.recent).toBe(10);
        expect(windows.previous).toBe(4);
        expect(windows.previousCount).toBe(3);
    });

    it('gives an odd payload the extra cycle to the newer half', () => {
        const entries = [1, 2, 3, 4, 5].map(n => velocity(`S${n}`, n * 2, n));

        const windows = SprintAnalyticsConverter.toVelocityWindows(entries, SprintUnit.Points);

        expect(windows.previousCount).toBe(2);
        expect(windows.recent).toBe(8);
        expect(windows.previous).toBe(3);
    });

    it('names the real size of the older window', () => {
        const entries = [1, 2, 3, 4, 5, 6, 7].map(n => velocity(`S${n}`, n * 2, n));

        const windows = SprintAnalyticsConverter.toVelocityWindows(entries, SprintUnit.Points);

        expect(windows.previousCount).toBe(3);
        expect(windows.previous).toBe(4);
        expect(windows.recent).toBe(11);
    });

    it('ignores cycles that scored zero in the unit being shown', () => {
        const entries = [velocity('A', 0, 0), velocity('B', 10, 5)];

        const windows = SprintAnalyticsConverter.toVelocityWindows(entries, SprintUnit.Points);

        expect(windows.recent).toBe(10);
    });

    it('counts only the cycles the older average was built from', () => {
        const entries = [0, 0, 0, 4, 5, 6, 7].map((n, index) => velocity(`S${index}`, n * 2, n));

        const windows = SprintAnalyticsConverter.toVelocityWindows(entries, SprintUnit.Points);

        expect(windows.previousCount).toBe(0);
        expect(windows.previous).toBeNull();
    });

    it('has no average at all without closed cycles', () => {
        const windows = SprintAnalyticsConverter.toVelocityWindows([], SprintUnit.Points);

        expect(windows.recent).toBeNull();
        expect(windows.previousCount).toBe(0);
    });
});

describe('SprintAnalyticsConverter.toRecentAverages', () => {
    it('averages the same slice the trend line calls recent', () => {
        const entries = [1, 2, 3, 4, 5, 6, 7].map(n => velocity(`S${n}`, n * 2, n));

        const averages = SprintAnalyticsConverter.toRecentAverages(entries)!;
        const windows = SprintAnalyticsConverter.toVelocityWindows(entries, SprintUnit.Points);

        expect(averages.points).toBe(windows.recent);
        expect(averages.issues).toBe(5.5);
    });

    it('is null without closed cycles', () => {
        expect(SprintAnalyticsConverter.toRecentAverages([])).toBeNull();
    });
});
