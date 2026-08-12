import { describe, expect, it } from 'vitest';
import { SprintState } from '../constants/sprint-state.enum';
import { SprintPhase } from '../constants/sprint-phase.enum';
import { SprintUnit } from '../constants/sprint-unit.enum';
import { Sprint } from '../model/sprint.model';
import { SprintVelocityAverages } from '../model/sprint-velocity.model';
import { SprintStats } from '../model/sprint-stats.model';
import {
    SprintHealthPlanned,
    SprintHealthRunning,
    SprintHealthClosed
} from '../entity/sprint-health.entity';
import { SprintHealthService } from './sprint-health.service';

function sprint(startAt: string, endAt: string, state: SprintState = SprintState.Planned): Sprint {
    return { idSprint: 1, idProject: 1, name: 'Sprint 4', startAt, endAt, state };
}

function stats(partial: Partial<SprintStats>): SprintStats {
    return {
        totalPoints: 0,
        donePoints: 0,
        startPoints: 0,
        progressPoints: 0,
        totalIssues: 0,
        doneIssues: 0,
        startIssues: 0,
        progressIssues: 0,
        pointedIssues: 1,
        ...partial
    };
}

function avg(points: number, issues = points): SprintVelocityAverages {
    return { points, issues };
}

const JUL_14 = '2026-07-14T00:00:00Z';
const JUL_28 = '2026-07-28T00:00:00Z';

function at(day: number): Date {
    return new Date(Date.UTC(2026, 6, 13 + day, 12, 0, 0));
}

describe('SprintHealthService', () => {
    const service = new SprintHealthService();

    function planned(
        partial: Partial<SprintStats>,
        velocity: SprintVelocityAverages | null,
        now = at(-2)
    ): SprintHealthPlanned {
        const health = service.toHealth(
            sprint(JUL_14, JUL_28),
            stats(partial),
            velocity,
            SprintUnit.Points,
            now
        );
        if (health.phase !== SprintPhase.NotStarted) {
            throw new Error(`expected a planned sprint, got ${health.phase}`);
        }
        return health;
    }

    function running(partial: Partial<SprintStats>, now: Date, end = JUL_28): SprintHealthRunning {
        const health = service.toHealth(
            sprint(JUL_14, end),
            stats(partial),
            null,
            SprintUnit.Points,
            now
        );
        if (health.phase !== SprintPhase.Running) {
            throw new Error(`expected a running sprint, got ${health.phase}`);
        }
        return health;
    }

    function closed(
        partial: Partial<SprintStats>,
        velocity: SprintVelocityAverages | null
    ): SprintHealthClosed {
        const health = service.toHealth(
            sprint(JUL_14, JUL_28, SprintState.Closed),
            stats(partial),
            velocity,
            SprintUnit.Points,
            at(20)
        );
        if (health.phase !== SprintPhase.Closed) {
            throw new Error(`expected a closed sprint, got ${health.phase}`);
        }
        return health;
    }

    describe('not started', () => {
        it('flags over-commitment above 1.15x the average', () => {
            const health = planned({ totalPoints: 12, pointedIssues: 3 }, avg(10));
            expect(health.isOverCommitted).toBe(true);
            expect(health.avgVelocity).toBe(10);
        });

        it('treats exactly 1.15x as within budget', () => {
            expect(planned({ totalPoints: 23, pointedIssues: 3 }, avg(20)).isOverCommitted).toBe(
                false
            );
        });

        it('has no verdict without an average', () => {
            const health = planned({ totalPoints: 99, pointedIssues: 3 }, null);
            expect(health.isOverCommitted).toBe(false);
            expect(health.avgVelocity).toBeNull();
        });

        it('never compares against an average of zero', () => {
            expect(planned({ totalPoints: 8, pointedIssues: 3 }, avg(0)).isOverCommitted).toBe(
                false
            );
        });
    });

    describe('running', () => {
        it('forecasts nothing before day 3', () => {
            const health = running({ totalPoints: 21, donePoints: 1, pointedIssues: 3 }, at(2));
            expect(health.isTooEarly).toBe(true);
            expect(health.forecast).toBeNull();
        });

        it('reproduces the plan worked example', () => {
            const health = running({ totalPoints: 21, donePoints: 13, pointedIssues: 3 }, at(10));
            expect(health.daysTotal).toBe(14);
            expect(health.daysElapsed).toBe(10);
            expect(health.forecast?.paceSoFar).toBeCloseTo(1.3, 5);
            expect(health.forecast?.paceNeeded).toBeCloseTo(2, 5);
            expect(health.forecast?.isOnTrack).toBe(false);
            expect(health.forecast?.gap).toBe(3);
        });

        it('is on track when the projection reaches the commitment', () => {
            const health = running({ totalPoints: 21, donePoints: 16, pointedIssues: 3 }, at(10));
            expect(health.forecast?.isOnTrack).toBe(true);
        });

        it('is on track at exactly 0.95x the commitment', () => {
            const health = running({ totalPoints: 40, donePoints: 19, pointedIssues: 3 }, at(7));
            expect(health.forecast?.isOnTrack).toBe(true);
        });

        it('never reports a gap of zero while off track', () => {
            const health = running({ totalPoints: 7, donePoints: 3, pointedIssues: 3 }, at(10));
            expect(health.forecast?.isOnTrack).toBe(false);
            expect(health.forecast?.gap).toBeGreaterThanOrEqual(1);
        });

        it('reports no gap at zero pace, rather than a bogus projection', () => {
            const health = running({ totalPoints: 21, donePoints: 0, pointedIssues: 3 }, at(5));
            expect(health.forecast?.paceSoFar).toBe(0);
            expect(health.forecast?.isOnTrack).toBe(false);
            expect(health.forecast?.gap).toBe(21);
        });

        it('is on track once the commitment is met', () => {
            const health = running({ totalPoints: 21, donePoints: 21, pointedIssues: 3 }, at(10));
            expect(health.forecast?.isOnTrack).toBe(true);
            expect(health.forecast?.gap).toBe(0);
        });

        it('keeps paceNeeded finite past the sprint end', () => {
            const health = running({ totalPoints: 21, donePoints: 5, pointedIssues: 3 }, at(20));
            expect(health.daysLeft).toBe(0);
            expect(health.daysElapsed).toBe(14);
            expect(Number.isFinite(health.forecast?.paceNeeded)).toBe(true);
            expect(health.forecast?.isOnTrack).toBe(false);
        });

        it('never lets daysElapsed exceed daysTotal', () => {
            const health = running({ totalPoints: 1, pointedIssues: 1 }, at(40));
            expect(health.daysElapsed).toBeLessThanOrEqual(health.daysTotal);
        });

        it('buckets by UTC dates regardless of the runner timezone', () => {
            const health = service.toHealth(
                sprint('2026-07-14T23:00:00Z', JUL_28),
                stats({ totalPoints: 10, pointedIssues: 2 }),
                null,
                SprintUnit.Points,
                new Date('2026-07-15T01:00:00Z')
            );
            expect(health.phase).toBe(SprintPhase.Running);
            expect((health as SprintHealthRunning).daysElapsed).toBe(2);
        });

        it('survives a zero commitment', () => {
            const health = running({ totalPoints: 0, pointedIssues: 2 }, at(10));
            expect(health.committed).toBe(0);
            expect(health.isTooEarly).toBe(false);
            expect(health.forecast).toBeNull();
        });

        it('survives a single-day sprint', () => {
            const health = running(
                { totalPoints: 3, donePoints: 1, pointedIssues: 1 },
                at(1),
                JUL_14
            );
            expect(health.daysTotal).toBe(1);
            expect(health.daysElapsed).toBe(1);
            expect(health.done).toBe(1);
            expect(health.committed).toBe(3);
        });
    });

    describe('closed', () => {
        it('renders the degraded phase-1 state', () => {
            const health = closed(
                {
                    totalPoints: 14,
                    donePoints: 14,
                    totalIssues: 6,
                    doneIssues: 6,
                    pointedIssues: 6
                },
                avg(10)
            );
            expect(health.committed).toBe(health.done);
            expect(health.notStarted).toBe(0);
        });

        it('forces tasks when the closed sprint has no points', () => {
            const health = closed({ totalIssues: 6, doneIssues: 6, pointedIssues: 0 }, null);
            expect(health.unit).toBe(SprintUnit.Issues);
            expect(health.isUnitForced).toBe(true);
            expect(health.done).toBe(6);
        });
    });

    describe('forced unit', () => {
        it('is not forced when the cycle holds no work at all', () => {
            const health = running({ totalIssues: 0, totalPoints: 0, pointedIssues: 0 }, at(10));
            expect(health.isUnitForced).toBe(false);
            expect(health.unit).toBe(SprintUnit.Points);
        });

        it('is not forced while the stats have not arrived yet', () => {
            const health = service.toHealth(
                sprint(JUL_14, JUL_28),
                null,
                null,
                SprintUnit.Points,
                at(5)
            );
            expect(health.isUnitForced).toBe(false);
            expect(health.unit).toBe(SprintUnit.Points);
        });

        it('is forced only when there is work and none of it is pointed, and counts tasks', () => {
            const health = running({ totalIssues: 20, doneIssues: 9, pointedIssues: 0 }, at(10));
            expect(health.isUnitForced).toBe(true);
            expect(health.unit).toBe(SprintUnit.Issues);
            expect(health.committed).toBe(20);
            expect(health.done).toBe(9);
        });
    });

    describe('backlog', () => {
        it('counts only', () => {
            const health = service.toHealth(
                null,
                stats({ totalPoints: 96, totalIssues: 42, pointedIssues: 20 }),
                avg(10),
                SprintUnit.Points,
                at(10)
            );
            expect(health.phase).toBe(SprintPhase.Backlog);
            expect(health.committed).toBe(96);
        });

        it('leaves the unit alone when nothing in the backlog is pointed', () => {
            const health = service.toHealth(
                null,
                stats({ totalIssues: 42, pointedIssues: 0 }),
                avg(10),
                SprintUnit.Points,
                at(10)
            );
            expect(health.isUnitForced).toBe(false);
            expect(health.unit).toBe(SprintUnit.Points);
        });
    });
});
