import { Injectable } from '@angular/core';
import { SprintPhase } from '../constants/sprint-phase.enum';
import { SprintUnit } from '../constants/sprint-unit.enum';
import { DAY_MS, DateUtil } from 'src/app/shared/date/date.util';
import { SprintState } from '../constants/sprint-state.enum';
import { Sprint } from '../model/sprint.model';
import { SprintStats } from '../model/sprint-stats.model';
import {
    SprintForecastFinish,
    SprintHealth,
    SprintHealthBase,
    SprintHealthClosed,
    SprintHealthForecast,
    SprintHealthPlanned,
    SprintHealthRunning
} from '../entity/sprint-health.entity';
import { SprintVelocityAverages } from '../model/sprint-velocity.model';

const OVER_COMMIT_RATIO = 1.15;
const ON_TRACK_RATIO = 0.95;
const FORECAST_MIN_DAYS = 3;

interface SprintWindow {
    daysTotal: number;
    daysElapsed: number;
    daysLeft: number;
    hasStarted: boolean;
}

@Injectable({ providedIn: 'root' })
export class SprintHealthService {
    public toHealth(
        sprint: Sprint | null,
        stats: SprintStats | null,
        avgVelocity: SprintVelocityAverages | null,
        unit: SprintUnit,
        now: Date
    ): SprintHealth {
        if (sprint === null) {
            return { ...this.toBase(stats, unit, false), phase: SprintPhase.Backlog };
        }
        const base = this.toBase(stats, unit, true);
        const window = this.toWindow(sprint, now);
        if (sprint.state === SprintState.Closed) {
            return this.toClosed(base, stats, unit);
        }
        if (!window.hasStarted) {
            return this.toPlanned(base, avgVelocity);
        }
        return this.toRunning(base, window);
    }

    public forcedUnit(stats: SprintStats | null, unit: SprintUnit): SprintUnit {
        return this.isUnitForced(stats) ? SprintUnit.Issues : unit;
    }

    private isUnitForced(stats: SprintStats | null): boolean {
        return (stats?.totalIssues ?? 0) > 0 && stats?.pointedIssues === 0;
    }

    private toBase(
        stats: SprintStats | null,
        unit: SprintUnit,
        canForceUnit: boolean
    ): SprintHealthBase {
        const isUnitForced = canForceUnit && this.isUnitForced(stats);
        const effectiveUnit = isUnitForced ? SprintUnit.Issues : unit;
        const usePoints = effectiveUnit === SprintUnit.Points;

        const committed = stats ? (usePoints ? stats.totalPoints : stats.totalIssues) : 0;
        const done = stats ? (usePoints ? stats.donePoints : stats.doneIssues) : 0;
        const inProgress = stats ? (usePoints ? stats.progressPoints : stats.progressIssues) : 0;
        const notStarted = stats ? (usePoints ? stats.startPoints : stats.startIssues) : 0;

        return {
            unit: effectiveUnit,
            isUnitForced,
            committed,
            done,
            inProgress,
            notStarted
        };
    }

    private toWindow(sprint: Sprint, now: Date): SprintWindow {
        const startDay = DateUtil.truncateTimeUtc(sprint.startAt).getTime();
        const endDay = DateUtil.truncateTimeUtc(sprint.endAt).getTime();
        const nowDay = DateUtil.truncateTimeUtc(now).getTime();
        const daysTotal = Math.max(this.dayDiff(endDay, startDay), 1);
        const daysElapsed = Math.min(Math.max(this.dayDiff(nowDay, startDay) + 1, 1), daysTotal);
        return {
            daysTotal,
            daysElapsed,
            daysLeft: Math.max(daysTotal - daysElapsed, 0),
            hasStarted: nowDay >= startDay
        };
    }

    private toClosed(
        base: SprintHealthBase,
        stats: SprintStats | null,
        unit: SprintUnit
    ): SprintHealthClosed {
        const rolledOverIssues = stats?.rolledOverIssues ?? null;
        if (stats?.frozenTotalIssues == null || stats.frozenPointedIssues == null) {
            return {
                ...base,
                phase: SprintPhase.Closed,
                isFrozen: false,
                rolledOverIssues,
                committed: base.done,
                inProgress: 0,
                notStarted: 0
            };
        }
        const isUnitForced = stats.frozenTotalIssues > 0 && stats.frozenPointedIssues === 0;
        const effectiveUnit = isUnitForced ? SprintUnit.Issues : unit;
        const usePoints = effectiveUnit === SprintUnit.Points;
        const committed = usePoints ? (stats.frozenTotalPoints ?? 0) : stats.frozenTotalIssues;
        const done = usePoints ? (stats.frozenDonePoints ?? 0) : (stats.frozenDoneIssues ?? 0);
        return {
            ...base,
            unit: effectiveUnit,
            isUnitForced,
            committed,
            done,
            inProgress: 0,
            notStarted: Math.max(committed - done, 0),
            phase: SprintPhase.Closed,
            isFrozen: true,
            rolledOverIssues
        };
    }

    public toForecastFinish(sprint: Sprint, health: SprintHealth): SprintForecastFinish | null {
        if (health.phase !== SprintPhase.Running || health.forecast === null) {
            return null;
        }
        const remaining = health.committed - health.done;
        if (remaining <= 0 || health.forecast.paceSoFar === 0) {
            return null;
        }
        const finishDay = health.daysElapsed + Math.ceil(remaining / health.forecast.paceSoFar);
        const daysLate = Math.max(finishDay - health.daysTotal, 0);
        const anchor =
            daysLate > 0
                ? DateUtil.truncateTimeUtc(sprint.endAt).getTime() + daysLate * DAY_MS
                : DateUtil.truncateTimeUtc(sprint.startAt).getTime() + (finishDay - 1) * DAY_MS;
        return { finish: new Date(anchor), daysLate };
    }

    private toPlanned(
        base: SprintHealthBase,
        avgVelocity: SprintVelocityAverages | null
    ): SprintHealthPlanned {
        const avg = avgVelocity
            ? base.unit === SprintUnit.Points
                ? avgVelocity.points
                : avgVelocity.issues
            : null;
        return {
            ...base,
            phase: SprintPhase.NotStarted,
            avgVelocity: avg,
            isOverCommitted: avg !== null && avg > 0 && base.committed > OVER_COMMIT_RATIO * avg
        };
    }

    private toRunning(base: SprintHealthBase, window: SprintWindow): SprintHealthRunning {
        const running = {
            ...base,
            phase: SprintPhase.Running as const,
            daysTotal: window.daysTotal,
            daysElapsed: window.daysElapsed,
            daysLeft: window.daysLeft
        };
        if (base.committed === 0) {
            return { ...running, isTooEarly: false, forecast: null };
        }
        if (window.daysElapsed < FORECAST_MIN_DAYS) {
            return { ...running, isTooEarly: true, forecast: null };
        }
        return { ...running, isTooEarly: false, forecast: this.toForecast(base, window) };
    }

    private toForecast(base: SprintHealthBase, window: SprintWindow): SprintHealthForecast {
        const paceSoFar = base.done / window.daysElapsed;
        const paceNeeded = (base.committed - base.done) / Math.max(window.daysLeft, 1);
        const projected = base.done + paceSoFar * window.daysLeft;
        const isOnTrack = projected >= ON_TRACK_RATIO * base.committed;
        return {
            isOnTrack,
            paceSoFar,
            paceNeeded,
            gap: isOnTrack ? 0 : Math.max(Math.round(base.committed - projected), 1)
        };
    }

    private dayDiff(a: number, b: number): number {
        return (a - b) / DAY_MS;
    }
}
