import { DestroyRef, Injectable, Signal, computed, inject } from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { HttpErrorResponse } from '@angular/common/http';
import { concat, EMPTY, merge, Observable, of, Subject, throwError, timer } from 'rxjs';
import {
    auditTime,
    catchError,
    debounceTime,
    filter,
    map,
    retry,
    switchMap,
    tap
} from 'rxjs/operators';
import { SprintApi } from '../api/sprint.api.service';
import { LoadState, LoadStateUtil } from 'src/app/shared/model/load-state.model';
import { SprintBurndown } from '../model/sprint-burndown.model';
import { SprintStats } from '../model/sprint-stats.model';
import { SprintVelocity } from '../model/sprint-velocity.model';

export const STATS_DEBOUNCE_MS = 300;

export const STATS_RETRY_MS = 1_000;

export const BURNDOWN_DEBOUNCE_MS = 2_000;

interface SprintStatsScope {
    idProject: number;
    idSprint: number | null;
}

interface SprintStatsRequest {
    scope: SprintStatsScope;
    isScopeChange: boolean;
}

interface SprintBurndownRequest {
    idSprint: number | null;
    isScopeChange: boolean;
}

@Injectable()
export class SprintAnalyticsStore {
    private readonly sprintApi = inject(SprintApi);

    private readonly destroyRef = inject(DestroyRef);

    private readonly statsRequest$ = new Subject<SprintStatsRequest>();

    private readonly velocityRequest$ = new Subject<number>();

    private readonly burndownNotice$ = new Subject<void>();

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

    private readonly burndownRequest$ = new Subject<SprintBurndownRequest>();

    private burndownScope: number | null = null;

    private latestBurndown: SprintBurndown | null = null;

    private readonly burndownState: Signal<LoadState<SprintBurndown>> = toSignal(
        this.burndownRequest$.pipe(switchMap(request => this.loadBurndown$(request))),
        { initialValue: LoadStateUtil.idle<SprintBurndown>() }
    );

    public readonly burndown = computed(() => this.burndownState().data);

    public readonly isBurndownLoading = computed(() => this.burndownState().isLoading);

    public reloadBurndown(): void {
        const idSprint = this.scope.idSprint;
        const isScopeChange = idSprint !== this.burndownScope;
        this.burndownScope = idSprint;
        this.burndownRequest$.next({ idSprint, isScopeChange });
    }

    public reloadBurndownAfterNotice(): void {
        this.burndownNotice$.next();
    }

    public constructor() {
        this.burndownNotice$
            .pipe(auditTime(BURNDOWN_DEBOUNCE_MS), takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.reloadBurndown());
    }

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

    private loadBurndown$(request: SprintBurndownRequest): Observable<LoadState<SprintBurndown>> {
        if (request.idSprint === null) {
            this.latestBurndown = null;
            return of(LoadStateUtil.loaded<SprintBurndown>(null));
        }
        if (request.isScopeChange) {
            this.latestBurndown = null;
        }
        return concat(
            of(LoadStateUtil.loading(this.latestBurndown)),
            this.sprintApi.loadBurndown$(request.idSprint).pipe(
                tap(burndown => (this.latestBurndown = burndown)),
                map(burndown => LoadStateUtil.loaded(burndown)),
                catchError(() => of(LoadStateUtil.loaded(this.latestBurndown)))
            )
        );
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
