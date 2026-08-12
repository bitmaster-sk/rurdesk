import { HttpErrorResponse } from '@angular/common/http';
import { DestroyRef, Injector, runInInjectionContext } from '@angular/core';
import { defer, Observable, of, throwError } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { SprintApi } from '../api/sprint.api.service';
import { SprintStats } from '../model/sprint-stats.model';
import { SprintVelocity } from '../model/sprint-velocity.model';
import { SprintAnalyticsStore, STATS_RETRY_MS } from './sprint-analytics.store';

function makeStats(donePoints: number): SprintStats {
    return {
        totalPoints: 20,
        donePoints,
        startPoints: 0,
        progressPoints: 0,
        totalIssues: 4,
        doneIssues: 1,
        startIssues: 0,
        progressIssues: 0,
        pointedIssues: 4
    };
}

function makeVelocity(): SprintVelocity {
    return {
        idSprint: 4,
        name: 'Sprint 4',
        endAt: '2026-07-01T00:00:00Z',
        donePoints: 12,
        doneIssues: 6,
        frozen: false
    };
}

function setup(api: Partial<Record<keyof SprintApi, unknown>>): SprintAnalyticsStore {
    const injector = Injector.create({
        providers: [
            { provide: DestroyRef, useValue: { onDestroy: () => () => undefined } },
            { provide: SprintApi, useValue: api }
        ]
    });
    return runInInjectionContext(injector, () => new SprintAnalyticsStore());
}

describe('SprintAnalyticsStore', () => {
    function failingTimes(count: number, status = 500): () => Observable<SprintStats> {
        let attempts = 0;
        return () =>
            defer(() => {
                attempts += 1;
                return attempts <= count
                    ? throwError(() => new HttpErrorResponse({ status }))
                    : of(makeStats(7));
            });
    }

    it('retries a failed stats request once, and shows the retry result', () => {
        vi.useFakeTimers();
        try {
            const store = setup({
                loadSprintStats$: vi.fn(failingTimes(1)),
                loadBacklogStats$: vi.fn()
            });

            store.scopeAndReload(1, 5);
            expect(store.stats()).toBeNull();

            vi.advanceTimersByTime(STATS_RETRY_MS);

            expect(store.stats()?.donePoints).toBe(7);
        } finally {
            vi.useRealTimers();
        }
    });

    it('does not retry a cycle that is gone, since the answer cannot change', () => {
        vi.useFakeTimers();
        try {
            const loadSprintStats$ = vi.fn(failingTimes(1, 404));
            const store = setup({ loadSprintStats$, loadBacklogStats$: vi.fn() });

            store.scopeAndReload(1, 5);
            vi.advanceTimersByTime(STATS_RETRY_MS * 5);

            expect(store.stats()).toBeNull();
        } finally {
            vi.useRealTimers();
        }
    });

    it('keeps serving stats after a request fails for good', () => {
        vi.useFakeTimers();
        try {
            const store = setup({
                loadSprintStats$: vi.fn(failingTimes(2)),
                loadBacklogStats$: vi.fn()
            });

            store.scopeAndReload(1, 5);
            vi.advanceTimersByTime(STATS_RETRY_MS);
            expect(store.stats()).toBeNull();

            store.reloadStats();

            expect(store.stats()?.donePoints).toBe(7);
        } finally {
            vi.useRealTimers();
        }
    });

    it('keeps serving velocity after a failed request', () => {
        const loadVelocity$ = vi
            .fn()
            .mockReturnValueOnce(throwError(() => new Error('boom')))
            .mockReturnValueOnce(of([makeVelocity()]));
        const store = setup({ loadVelocity$ });

        store.setScope(1, null);
        store.reloadVelocity();
        expect(store.velocities()).toEqual([]);

        store.reloadVelocity();

        expect(store.velocities()).toHaveLength(1);
    });

    it('asks the backlog endpoint when the scope carries no sprint', () => {
        const loadSprintStats$ = vi.fn().mockReturnValue(of(makeStats(3)));
        const loadBacklogStats$ = vi.fn().mockReturnValue(of(makeStats(9)));
        const store = setup({ loadSprintStats$, loadBacklogStats$ });

        store.scopeAndReload(1, 5);
        expect(store.stats()?.donePoints).toBe(3);

        store.scopeAndReload(1, null);

        expect(loadBacklogStats$).toHaveBeenCalledWith(1);
        expect(store.stats()?.donePoints).toBe(9);
    });
});
