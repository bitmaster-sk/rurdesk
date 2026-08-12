import { Injectable, Signal, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { HttpErrorResponse } from '@angular/common/http';
import { concat, EMPTY, merge, Observable, of, Subject, throwError, timer } from 'rxjs';
import { catchError, debounceTime, filter, map, retry, switchMap } from 'rxjs/operators';
import { SprintApi } from '../api/sprint.api.service';
import { SprintStats } from '../model/sprint-stats.model';
import { SprintVelocity } from '../model/sprint-velocity.model';

export const STATS_DEBOUNCE_MS = 300;

export const STATS_RETRY_MS = 1_000;

interface SprintStatsScope {
    idProject: number;
    idSprint: number | null;
}

interface SprintStatsRequest {
    scope: SprintStatsScope;
    isScopeChange: boolean;
}

@Injectable()
export class SprintAnalyticsStore {
    private readonly sprintApi = inject(SprintApi);

    private readonly statsRequest$ = new Subject<SprintStatsRequest>();

    private readonly velocityRequest$ = new Subject<number>();

    private readonly noticedChange$ = new Subject<void>();

    private scope: SprintStatsScope = { idProject: 0, idSprint: null };

    public readonly stats: Signal<SprintStats | null> = toSignal(
        merge(
            this.statsRequest$,
            this.noticedChange$.pipe(
                debounceTime(STATS_DEBOUNCE_MS),
                map(() => ({ scope: this.scope, isScopeChange: false }))
            )
        ).pipe(
            filter(request => request.scope.idProject > 0),
            switchMap(request =>
                request.isScopeChange
                    ? concat(of(null), this.loadStats$(request.scope))
                    : this.loadStats$(request.scope)
            )
        ),
        { initialValue: null }
    );

    public readonly velocities: Signal<SprintVelocity[]> = toSignal(
        this.velocityRequest$.pipe(
            filter(idProject => idProject > 0),
            switchMap(idProject =>
                this.sprintApi.loadVelocity$(idProject).pipe(catchError(() => EMPTY))
            )
        ),
        { initialValue: [] }
    );

    public setScope(idProject: number, idSprint: number | null): void {
        this.scope = { idProject, idSprint };
    }

    public scopeAndReload(idProject: number, idSprint: number | null): void {
        this.setScope(idProject, idSprint);
        this.statsRequest$.next({ scope: this.scope, isScopeChange: true });
    }

    public reloadStats(): void {
        this.statsRequest$.next({ scope: this.scope, isScopeChange: false });
    }

    public reloadVelocity(): void {
        this.velocityRequest$.next(this.scope.idProject);
    }

    public reloadStatsAfterNotice(): void {
        this.noticedChange$.next();
    }

    private loadStats$(scope: SprintStatsScope): Observable<SprintStats> {
        const request$ =
            scope.idSprint === null
                ? this.sprintApi.loadBacklogStats$(scope.idProject)
                : this.sprintApi.loadSprintStats$(scope.idSprint);
        return request$.pipe(
            retry({
                count: 1,
                delay: (error: HttpErrorResponse) =>
                    error.status >= 500 ? timer(STATS_RETRY_MS) : throwError(() => error)
            }),
            catchError(() => EMPTY)
        );
    }
}
