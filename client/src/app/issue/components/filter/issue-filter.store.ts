import { Injectable } from '@angular/core';
import { BehaviorSubject, Subject } from 'rxjs';
import { filter, map } from 'rxjs/operators';
import { IssuesFilter, IssuesFilterParams, IssuesOrderParams } from './issue-filter.entity';

@Injectable()
export class IssueFilterStore {
    private filter = new BehaviorSubject<{
        filter: IssuesFilter | null;
        initial: boolean;
        refresh: boolean;
    }>({ filter: null, initial: false, refresh: false });

    private showFilter = new BehaviorSubject<boolean>(false);

    public actualFilter$ = this.filter.asObservable().pipe(
        map(f => f.filter),
        filter((f): f is IssuesFilter => !!f)
    );

    // Same emissions as actualFilter$, but carries whether this is a pure data refresh
    // (server data changed — paginated views must KEEP their loaded pages) or a real
    // filter/order change (reset pagination to page 1). Paginated views (gantt backlog,
    // kanban columns) subscribe here so a refresh() after a mutation keeps their current
    // page instead of resetting to page 1.
    public actualFilterChange$ = this.filter.asObservable().pipe(
        filter(
            (f): f is { filter: IssuesFilter; initial: boolean; refresh: boolean } => !!f.filter
        ),
        map(f => ({ filter: f.filter, refresh: f.refresh }))
    );

    public initialFilter$ = this.filter.asObservable().pipe(
        filter(f => f.initial),
        map(f => f.filter),
        filter((f): f is IssuesFilter => !!f)
    );

    public showFilter$ = this.showFilter.asObservable();

    /** Should emit only when the filter is modified (not initial or refresh) */
    private readonly _isFilterEdited = new Subject<void>();
    public readonly isFilterEdited$ = this._isFilterEdited.asObservable();

    public setInitialFilter(initialFilter: IssuesFilter): void {
        this.filter.next({ filter: initialFilter, initial: true, refresh: false });
    }

    public setOrder(orderParams: IssuesOrderParams): void {
        const actualFilter = this.filter.getValue()?.filter;
        const newFilter = { ...actualFilter, ...orderParams } as IssuesFilter;
        this.filter.next({ initial: false, filter: newFilter, refresh: false });
        this._isFilterEdited.next();
    }

    // idSprint === null means the Backlog tab → filter to issues with no sprint.
    public setSprint(idSprint: number | null): void {
        const actualFilter = this.filter.getValue()?.filter;
        const newFilter = {
            ...actualFilter,
            idSprint,
            sprintUnset: idSprint === null
        } as IssuesFilter;
        this.filter.next({ initial: false, filter: newFilter, refresh: false });
    }

    public getFilter(): IssuesFilter | null {
        return this.filter.getValue().filter;
    }

    public setFilter(filterParams: IssuesFilterParams): void {
        const actualFilter = this.filter.getValue()?.filter;
        const newFilter = { ...actualFilter, ...filterParams } as IssuesFilter;

        this.filter.next({ filter: newFilter, initial: false, refresh: false });
        this._isFilterEdited.next();
    }

    public toggleShowFilter(): void {
        this.showFilter.next(!this.showFilter.getValue());
    }

    public setShowFilter(showFilter: boolean): void {
        this.showFilter.next(showFilter);
    }

    public refresh(): void {
        const current = this.filter.getValue();
        this.filter.next({ ...current, refresh: true });
    }

    // Reset to an empty filter. Called by each view when it mounts so the new view's
    // load stream does not fire on the *previous* view's leftover filter (a stale
    // cross-view load) before this view's setInitialFilter runs. actualFilter$ /
    // actualFilterChange$ drop a null filter, so nothing loads until setInitialFilter.
    public clear(): void {
        this.filter.next({ filter: null, initial: false, refresh: false });
    }
}
