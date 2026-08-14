import { DAY_MS, DateUtil } from 'src/app/shared/date/date.util';
import { SprintUnit } from '../constants/sprint-unit.enum';
import { SprintBurndown, SprintBurndownDay } from '../model/sprint-burndown.model';
import { SprintVelocity, SprintVelocityAverages } from '../model/sprint-velocity.model';

export interface SprintBurndownSeries {
    labels: string[];
    remaining: (number | null)[];
    done: (number | null)[];
    scope: (number | null)[];
    isReal: boolean[];
}

export interface SprintScopeChange {
    delta: number;
    day: string | null;
    dayIndex: number | null;
    daysTotal: number;
}

export interface SprintVelocitySeries {
    labels: string[];
    done: number[];
    planned: (number | null)[];
}

export interface SprintVelocityWindows {
    recent: number | null;
    previous: number | null;
    previousCount: number;
}

interface ScopePoint {
    day: string;
    scope: number;
}

export abstract class SprintAnalyticsConverter {
    public static toBurndownSeries(
        burndown: SprintBurndown | null,
        unit: SprintUnit
    ): SprintBurndownSeries {
        const days = burndown?.days ?? [];
        return {
            labels: days.map(day => day.day),
            remaining: days.map(day => this.remainingOf(day, unit)),
            done: days.map(day => this.doneOf(day, unit)),
            scope: days.map(day => this.scopeOf(day, unit)),
            isReal: days.map(day => day.snapshot)
        };
    }

    public static toIdealLine(
        burndown: SprintBurndown | null,
        unit: SprintUnit
    ): (number | null)[] {
        const days = burndown?.days ?? [];
        if (days.length === 0) {
            return [];
        }
        const baseline = this.baselineOf(burndown, unit);
        if (baseline === null) {
            return days.map(() => null);
        }
        const endIndex = this.endIndexOf(burndown!, days);
        return days.map((_, index) => {
            if (index > endIndex) {
                return null;
            }
            return endIndex === 0 ? baseline.scope : baseline.scope * (1 - index / endIndex);
        });
    }

    public static toScopeAdded(
        burndown: SprintBurndown | null,
        unit: SprintUnit
    ): SprintScopeChange {
        const points = this.scopeSequenceOf(burndown, unit);
        const daysTotal = burndown
            ? Math.max(this.dayDiff(burndown.endAt, burndown.startAt), 1)
            : 0;
        if (points.length === 0) {
            return { delta: 0, day: null, dayIndex: null, daysTotal };
        }
        const delta = points[points.length - 1].scope - points[0].scope;
        if (delta === 0 || points.length < 2) {
            return { delta, day: null, dayIndex: null, daysTotal };
        }

        let picked: ScopePoint | null = null;
        let best = 0;
        for (let index = 1; index < points.length; index++) {
            const change = points[index].scope - points[index - 1].scope;
            if (delta > 0 ? change > best : change < best) {
                best = change;
                picked = points[index];
            }
        }
        if (picked === null) {
            return { delta, day: null, dayIndex: null, daysTotal };
        }
        return {
            delta,
            day: picked.day,
            dayIndex: this.dayDiff(picked.day, burndown!.startAt) + 1,
            daysTotal
        };
    }

    public static toVelocitySeries(
        entries: SprintVelocity[],
        unit: SprintUnit
    ): SprintVelocitySeries {
        const usePoints = unit === SprintUnit.Points;
        return {
            labels: entries.map(entry => entry.name),
            done: entries.map(entry => (usePoints ? entry.donePoints : entry.doneIssues)),
            planned: entries.map(entry => {
                const planned = usePoints ? entry.plannedPoints : entry.plannedIssues;
                return entry.frozen && planned !== undefined ? planned : null;
            })
        };
    }

    public static toVelocityWindows(
        entries: SprintVelocity[],
        unit: SprintUnit
    ): SprintVelocityWindows {
        const recentWindow = this.recentWindow(entries);
        const previousWindow = entries.slice(0, entries.length - recentWindow.length);
        const recent = this.averageOfScored(this.scoresOf(recentWindow, unit));
        const previous = this.averageOfScored(this.scoresOf(previousWindow, unit));
        return {
            recent: recent.count === 0 ? null : recent.average,
            previous: previous.count === 0 ? null : previous.average,
            previousCount: previous.count
        };
    }

    public static toRecentAverages(entries: SprintVelocity[]): SprintVelocityAverages | null {
        const recent = this.recentWindow(entries);
        if (recent.length === 0) {
            return null;
        }
        return {
            points: this.averageOfScored(this.scoresOf(recent, SprintUnit.Points)).average,
            issues: this.averageOfScored(this.scoresOf(recent, SprintUnit.Issues)).average
        };
    }

    // The endpoint returns two windows' worth: the newer half is the configured one.
    private static recentWindow(entries: SprintVelocity[]): SprintVelocity[] {
        return entries.slice(Math.floor(entries.length / 2));
    }

    private static scoresOf(entries: SprintVelocity[], unit: SprintUnit): number[] {
        return entries.map(entry =>
            unit === SprintUnit.Points ? entry.donePoints : entry.doneIssues
        );
    }

    private static averageOfScored(values: number[]): { average: number; count: number } {
        const scored = values.filter(value => value > 0);
        return {
            average:
                scored.length === 0
                    ? 0
                    : scored.reduce((sum, value) => sum + value, 0) / scored.length,
            count: scored.length
        };
    }

    private static scopeSequenceOf(
        burndown: SprintBurndown | null,
        unit: SprintUnit
    ): ScopePoint[] {
        const points: ScopePoint[] = [];
        for (const day of burndown?.days ?? []) {
            const scope = this.scopeOf(day, unit);
            if (scope === null || (!day.snapshot && points.length > 0)) {
                continue;
            }
            points.push({ day: day.day, scope });
        }
        return points;
    }

    private static baselineOf(
        burndown: SprintBurndown | null,
        unit: SprintUnit
    ): ScopePoint | null {
        return this.scopeSequenceOf(burndown, unit)[0] ?? null;
    }

    private static endIndexOf(burndown: SprintBurndown, days: SprintBurndownDay[]): number {
        const endDay = DateUtil.truncateTimeUtc(burndown.endAt).getTime();
        const index = days.findIndex(day => DateUtil.truncateTimeUtc(day.day).getTime() === endDay);
        return index === -1 ? days.length - 1 : index;
    }

    private static scopeOf(day: SprintBurndownDay, unit: SprintUnit): number | null {
        return unit === SprintUnit.Points ? day.totalPoints : day.totalIssues;
    }

    private static doneOf(day: SprintBurndownDay, unit: SprintUnit): number | null {
        return unit === SprintUnit.Points ? day.donePoints : day.doneIssues;
    }

    private static remainingOf(day: SprintBurndownDay, unit: SprintUnit): number | null {
        return unit === SprintUnit.Points ? day.remainingPoints : day.remainingIssues;
    }

    private static dayDiff(a: string, b: string): number {
        return (
            (DateUtil.truncateTimeUtc(a).getTime() - DateUtil.truncateTimeUtc(b).getTime()) / DAY_MS
        );
    }
}
