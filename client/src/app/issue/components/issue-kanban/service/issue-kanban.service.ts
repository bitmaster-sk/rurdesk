import { inject, Injectable } from '@angular/core';
import { BehaviorSubject, Observable, forkJoin } from 'rxjs';
import { first, map, shareReplay, switchMap, tap } from 'rxjs/operators';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { StateStore } from 'src/app/state/store/state.store';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { IssueFilterStore } from '../../filter/issue-filter.store';
import { IssuesFilter } from '../../filter/issue-filter.entity';
import { IssueService } from '../../../issue.service';
import { Issue } from '../../../model/issue.model';
import { IssueGroup } from '../../../model/issues-page.model';
import { KanbanColumn } from '../entity/kanban-column.entity';
import { KanbanTile } from '../entity/kanban-tile.entity';
import { SwimlaneCell } from '../entity/swimlane-cell.entity';
import { SwimlaneRow } from '../entity/swimlane-row.entity';

interface StoreMaps {
    severities: Map<number, IssueSeverity>;
    states: Map<number, IssueState>;
    users: Map<number, User>;
}

@Injectable()
export class IssueKanbanService {
    private readonly issueService = inject(IssueService);
    private readonly stateStore = inject(StateStore);
    private readonly projectMemberStore = inject(ProjectMemberStore);
    private readonly severityStore = inject(SeverityStore);
    private readonly issueFilterStore = inject(IssueFilterStore);
    private readonly settings = inject(SettingsStore);

    // Latest filter + store maps, captured per load — used by per-column/cell "Load more".
    private ctx: ({ filter: IssuesFilter } & StoreMaps) | null = null;

    // Subjects let "Load more" re-emit a NEW array reference so the OnPush child re-renders
    // (mutating tiles in place would not trigger change detection).
    private readonly columnsSubject = new BehaviorSubject<KanbanColumn[]>([]);
    private readonly swimlaneSubject = new BehaviorSubject<SwimlaneRow[]>([]);

    constructor() {
        // Drop any leftover filter from a previously-mounted view so we don't fire a
        // stale load before this view's setInitialFilter runs.
        this.issueFilterStore.clear();
    }

    private maps$(filter: IssuesFilter): Observable<StoreMaps> {
        return forkJoin([
            this.severityStore.severitiesMapByProject$(filter.idProject).pipe(first()),
            this.stateStore.statesMapByProject$(filter.idProject).pipe(first()),
            this.projectMemberStore.usersMap$.pipe(first())
        ]).pipe(map(([severities, states, users]) => ({ severities, states, users })));
    }

    // Plain mode: one grouped request per state (top-N + total + cursor), then stream the subject
    // (so Load more pushes update the same stream).
    public readonly columns$ = this.issueFilterStore.actualFilterChange$.pipe(
        switchMap(({ filter, refresh }) =>
            forkJoin([
                this.issueService.loadIssuesGrouped$(
                    filter,
                    'state',
                    refresh ? this.columnExtent() : this.settings.kanbanPageSize()
                ),
                this.maps$(filter)
            ]).pipe(
                tap(([grouped, maps]) => {
                    this.ctx = { filter, ...maps };
                    this.columnsSubject.next(this.toColumns(grouped.groups, maps));
                }),
                switchMap(() => this.columnsSubject)
            )
        ),
        shareReplay({ bufferSize: 1, refCount: true })
    );

    // Swimlane mode: one grouped request per (state, assignedTo).
    public readonly swimlaneRows$ = this.issueFilterStore.actualFilterChange$.pipe(
        switchMap(({ filter, refresh }) =>
            forkJoin([
                this.issueService.loadIssuesGrouped$(
                    filter,
                    'state,assignedTo',
                    refresh ? this.swimlaneExtent() : this.settings.kanbanPageSize()
                ),
                this.maps$(filter)
            ]).pipe(
                tap(([grouped, maps]) => {
                    this.ctx = { filter, ...maps };
                    this.swimlaneSubject.next(this.toSwimlaneRows(grouped.groups, maps));
                }),
                switchMap(() => this.swimlaneSubject)
            )
        ),
        shareReplay({ bufferSize: 1, refCount: true })
    );

    // On a data refresh, re-request enough per group to cover the most-loaded column so
    // "Load more" progress survives. The grouped endpoint takes a single per-group limit,
    // so we size it to the widest column (others simply keep all they have).
    private columnExtent(): number {
        const tiles = this.columnsSubject.getValue().map(c => c.tiles.length);
        const loaded = tiles.length ? Math.max(...tiles) : 0;
        return Math.max(loaded, this.settings.kanbanPageSize());
    }

    private swimlaneExtent(): number {
        const tiles = this.swimlaneSubject
            .getValue()
            .flatMap(row => row.cells.map(c => c.tiles.length));
        const loaded = tiles.length ? Math.max(...tiles) : 0;
        return Math.max(loaded, this.settings.kanbanPageSize());
    }

    public readonly states$ = this.issueFilterStore.actualFilter$.pipe(
        switchMap(filter =>
            this.stateStore.statesMapByProject$(filter.idProject).pipe(
                first(),
                map(states => Array.from(states.values()))
            )
        ),
        shareReplay({ bufferSize: 1, refCount: true })
    );

    public loadMoreColumn(column: KanbanColumn): void {
        if (!this.ctx || !column.cursor || column.loading) return;
        const idState = column.state.idState;
        const colFilter: IssuesFilter = {
            ...this.ctx.filter,
            idsState: [idState],
            stateUnset: false
        };

        this.columnsSubject.next(
            this.columnsSubject
                .getValue()
                .map(c => (c.state.idState === idState ? { ...c, loading: true } : c))
        );

        this.issueService
            .loadIssuesPage$(colFilter, this.settings.kanbanPageSize(), column.cursor)
            .subscribe({
                next: page => {
                    const newTiles = page.items.map(issue => this.toTile(issue, this.ctx!));
                    const columns = this.columnsSubject.getValue().map(c =>
                        c.state.idState === idState
                            ? {
                                  ...c,
                                  tiles: [...c.tiles, ...newTiles],
                                  cursor: page.nextCursor,
                                  loading: false
                              }
                            : c
                    );
                    this.columnsSubject.next(columns);
                },
                // loadMoreColumn() returns early while `loading` is set, so the flag
                // must clear or this column cannot page again until the board is
                // rebuilt. The cursor is kept so a retry resumes where it stopped.
                error: () => this.clearColumnLoading(idState)
            });
    }

    private clearColumnLoading(idState: number): void {
        this.columnsSubject.next(
            this.columnsSubject
                .getValue()
                .map(c => (c.state.idState === idState ? { ...c, loading: false } : c))
        );
    }

    public loadMoreCell(cell: SwimlaneCell): void {
        if (!this.ctx || !cell.cursor || cell.loading) return;
        const idState = cell.state.idState;
        const idUser = cell.user?.idUser;
        const cellFilter: IssuesFilter = {
            ...this.ctx.filter,
            idsState: [idState],
            stateUnset: false,
            idsAssignedTo: cell.user ? [cell.user.idUser] : [],
            assignedToUnset: false,
            assignedToNull: !cell.user
        };
        const matches = (c: SwimlaneCell): boolean =>
            c.state.idState === idState && c.user?.idUser === idUser;

        this.swimlaneSubject.next(
            this.swimlaneSubject.getValue().map(row => ({
                ...row,
                cells: row.cells.map(c => (matches(c) ? { ...c, loading: true } : c))
            }))
        );

        this.issueService
            .loadIssuesPage$(cellFilter, this.settings.kanbanPageSize(), cell.cursor)
            .subscribe({
                next: page => {
                    const newTiles = page.items.map(issue => this.toTile(issue, this.ctx!));
                    const rows = this.swimlaneSubject.getValue().map(row => ({
                        ...row,
                        cells: row.cells.map(c =>
                            matches(c)
                                ? {
                                      ...c,
                                      tiles: [...c.tiles, ...newTiles],
                                      cursor: page.nextCursor,
                                      loading: false
                                  }
                                : c
                        )
                    }));
                    this.swimlaneSubject.next(rows);
                },
                error: () =>
                    this.swimlaneSubject.next(
                        this.swimlaneSubject.getValue().map(row => ({
                            ...row,
                            cells: row.cells.map(c => (matches(c) ? { ...c, loading: false } : c))
                        }))
                    )
            });
    }

    /**
     * Applies a live (WebSocket) issue change to the loaded board data in place
     * of a full reload, so the UI can animate the transition.
     *
     * Returns:
     * - `'moved'`   — the tile changed column/cell (state or assignee)
     * - `'updated'` — the tile was refreshed in place (or removed: left scope)
     * - `'missing'` — the issue isn't in the loaded data; caller should reload
     *
     * The verdict comes from the ACTIVE view's data only — the inactive view's
     * subject may hold stale rows from before a view switch, and a stale
     * "found it" there must not suppress the caller's reload. A client's own
     * optimistic drag already moved the tile, so the echo of that change lands
     * in the `'updated'` branch — no double animation.
     */
    public applyRemoteIssue(
        issue: Issue,
        idSprintScope: number | null,
        activeView: 'columns' | 'swimlane'
    ): 'moved' | 'updated' | 'missing' {
        if (!this.ctx) return 'missing';

        // A restrictive filter (state/severity/assignee) can't be re-evaluated
        // client-side against the payload — fall back to a server reload so a
        // card edited out of the filter doesn't linger on the board.
        if (this.hasRestrictiveFilter(this.ctx.filter)) return 'missing';

        const isInScope = (issue.idSprint ?? null) === idSprintScope;
        const tile = this.toTile(issue, this.ctx);

        const columnResult = this.applyToColumns(tile, isInScope);
        const swimlaneResult = this.applyToSwimlane(tile, isInScope);

        return activeView === 'columns' ? columnResult : swimlaneResult;
    }

    private hasRestrictiveFilter(filter: IssuesFilter): boolean {
        return (
            (!filter.stateUnset && (filter.idsState?.length ?? 0) > 0) ||
            (!filter.severityUnset && (filter.idsSeverity?.length ?? 0) > 0) ||
            (!filter.assignedToUnset && (filter.idsAssignedTo?.length ?? 0) > 0)
        );
    }

    private applyToColumns(tile: KanbanTile, isInScope: boolean): 'moved' | 'updated' | 'missing' {
        const columns = this.columnsSubject.getValue();
        if (columns.length === 0) return 'missing';

        const sourceColumn = columns.find(c => c.tiles.some(t => t.idIssue === tile.idIssue));
        if (!sourceColumn) return 'missing';

        // Left the board scope (sprint change / no state) → drop the tile
        if (!isInScope || tile.idState == null) {
            this.columnsSubject.next(
                columns.map(c =>
                    c === sourceColumn
                        ? {
                              ...c,
                              tiles: c.tiles.filter(t => t.idIssue !== tile.idIssue),
                              total: Math.max(0, c.total - 1)
                          }
                        : c
                )
            );
            return 'updated';
        }

        if (sourceColumn.state.idState === tile.idState) {
            this.columnsSubject.next(
                columns.map(c =>
                    c === sourceColumn
                        ? { ...c, tiles: c.tiles.map(t => (t.idIssue === tile.idIssue ? tile : t)) }
                        : c
                )
            );
            return 'updated';
        }

        const targetColumn = columns.find(c => c.state.idState === tile.idState);
        if (!targetColumn) return 'missing';
        this.columnsSubject.next(
            columns.map(c => {
                if (c === sourceColumn)
                    return {
                        ...c,
                        tiles: c.tiles.filter(t => t.idIssue !== tile.idIssue),
                        total: Math.max(0, c.total - 1)
                    };
                if (c === targetColumn)
                    return { ...c, tiles: [tile, ...c.tiles], total: c.total + 1 };
                return c;
            })
        );
        return 'moved';
    }

    private applyToSwimlane(tile: KanbanTile, isInScope: boolean): 'moved' | 'updated' | 'missing' {
        const rows = this.swimlaneSubject.getValue();
        if (rows.length === 0) return 'missing';

        let sourceCell: SwimlaneCell | null = null;
        for (const row of rows) {
            for (const cell of row.cells) {
                if (cell.tiles.some(t => t.idIssue === tile.idIssue)) sourceCell = cell;
            }
        }
        if (!sourceCell) return 'missing';

        const removeFrom = (cell: SwimlaneCell): SwimlaneCell => ({
            ...cell,
            tiles: cell.tiles.filter(t => t.idIssue !== tile.idIssue),
            total: Math.max(0, cell.total - 1)
        });

        if (!isInScope || tile.idState == null) {
            this.swimlaneSubject.next(
                rows.map(row => ({
                    ...row,
                    cells: row.cells.map(c => (c === sourceCell ? removeFrom(c) : c))
                }))
            );
            return 'updated';
        }

        const isTargetCell = (cell: SwimlaneCell): boolean =>
            cell.state.idState === tile.idState &&
            (cell.user?.idUser ?? null) === (tile.assignedTo ?? null);

        if (isTargetCell(sourceCell)) {
            this.swimlaneSubject.next(
                rows.map(row => ({
                    ...row,
                    cells: row.cells.map(c =>
                        c === sourceCell
                            ? {
                                  ...c,
                                  tiles: c.tiles.map(t => (t.idIssue === tile.idIssue ? tile : t))
                              }
                            : c
                    )
                }))
            );
            return 'updated';
        }

        // Target cell must exist (a brand-new assignee has no row yet → reload)
        const hasTarget = rows.some(row => row.cells.some(isTargetCell));
        if (!hasTarget) return 'missing';

        this.swimlaneSubject.next(
            rows.map(row => ({
                ...row,
                cells: row.cells.map(c => {
                    if (c === sourceCell) return removeFrom(c);
                    if (isTargetCell(c))
                        return { ...c, tiles: [tile, ...c.tiles], total: c.total + 1 };
                    return c;
                })
            }))
        );
        return 'moved';
    }

    private toColumns(groups: IssueGroup[], maps: StoreMaps): KanbanColumn[] {
        const byState = new Map<number, IssueGroup>();
        groups.forEach(g => {
            const idState = g.key['idState'];
            if (idState != null) byState.set(idState, g);
        });
        return Array.from(maps.states.values()).map(state => {
            const g = byState.get(state.idState);
            return {
                state,
                tiles: (g?.items ?? []).map(issue => this.toTile(issue, maps)),
                total: g?.total ?? 0,
                cursor: g?.nextCursor ?? null,
                loading: false
            };
        });
    }

    private toSwimlaneRows(groups: IssueGroup[], maps: StoreMaps): SwimlaneRow[] {
        const byKey = new Map<string, IssueGroup>();
        const userIds = new Set<number>();
        groups.forEach(g => {
            const idState = g.key['idState'];
            const assignedTo = g.key['assignedTo'];
            byKey.set(`${assignedTo ?? 'null'}|${idState}`, g);
            if (assignedTo != null) userIds.add(assignedTo);
        });

        const namedUsers = Array.from(userIds)
            .map(id => maps.users.get(id))
            .filter((u): u is User => !!u)
            .sort((a, b) => a.name.localeCompare(b.name));
        const distinctUsers: (User | undefined)[] = [undefined, ...namedUsers];

        const stateList = Array.from(maps.states.values());

        return distinctUsers.map(user => ({
            user,
            cells: stateList.map(state => {
                const g = byKey.get(`${user?.idUser ?? 'null'}|${state.idState}`);
                return {
                    state,
                    user,
                    tiles: (g?.items ?? []).map(issue => this.toTile(issue, maps)),
                    total: g?.total ?? 0,
                    cursor: g?.nextCursor ?? null,
                    loading: false
                };
            })
        }));
    }

    private toTile(issue: Issue, maps: StoreMaps): KanbanTile {
        return {
            ...issue,
            state: issue.idState != null ? maps.states.get(issue.idState) : undefined,
            severity: issue.idSeverity != null ? maps.severities.get(issue.idSeverity) : undefined,
            createUser: issue.createBy != null ? maps.users.get(issue.createBy) : undefined,
            updateUser: issue.updateBy != null ? maps.users.get(issue.updateBy) : undefined,
            assignedToUser: issue.assignedTo != null ? maps.users.get(issue.assignedTo) : undefined
        };
    }
}
