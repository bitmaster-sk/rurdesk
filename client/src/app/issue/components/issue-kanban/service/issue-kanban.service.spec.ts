import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { IssueKanbanService } from './issue-kanban.service';
import { IssueService } from '../../../issue.service';
import { IssuesPage } from '../../../model/issues-page.model';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { IssueFilterStore } from '../../filter/issue-filter.store';
import { IssuesFilter } from '../../filter/issue-filter.entity';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { SprintStore } from '../../../store/sprint.store';
import { SeverityStore } from 'src/app/severity/store/severity.store';
import { IssueTypeStore } from 'src/app/issue-type/store/issue-type.store';
import { StateStore } from 'src/app/state/store/state.store';
import { Issue } from '../../../model/issue.model';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { User } from 'src/app/auth/model/user.model';
import { IssueSeverity } from 'src/app/severity/model/issue-severity.model';
import { SwimlaneRow } from '../entity/swimlane-row.entity';
import { KanbanColumn } from '../entity/kanban-column.entity';

function initialFilter(): IssuesFilter {
    return { idProject: 1, orderColumn: 'idIssuePublic', orderDirection: 'desc' };
}

const stateA: IssueState = {
    idState: 1,
    name: 'Todo',
    idProject: 1,
    start: true,
    final: false,
    protected: false,
    orderRank: 0
};
const stateB: IssueState = {
    idState: 2,
    name: 'Done',
    idProject: 1,
    start: false,
    final: true,
    protected: false,
    orderRank: 1
};

const alice: User = { idUser: 10, name: 'Alice', email: 'a@a.com', colorAvatarBg: '#aaa' };
const bob: User = { idUser: 20, name: 'Bob', email: 'b@b.com', colorAvatarBg: '#bbb' };

const sev: IssueSeverity = {
    idSeverity: 1,
    idProject: 1,
    title: 'Low',
    color: '#ccc',
    protected: false,
    orderRank: 0
};

function makeIssue(overrides: Partial<Issue>): Issue {
    return {
        idIssue: 1,
        idIssuePublic: overrides.idIssue ?? 1,
        idProject: 1,
        idState: 1,
        idSeverity: 1,
        title: 'test',
        description: '',
        tracked: 0,
        createBy: 1,
        updateBy: 1,
        ...overrides
    };
}

// Mimics the backend grouped endpoint: groups issues by state (and assignedTo for swimlane).
function buildGroups(issues: Issue[], groupBy: string) {
    const swimlane = groupBy.includes('assignedTo');
    const map = new Map<string, { key: Record<string, number | null>; items: Issue[] }>();
    for (const i of issues) {
        const idState = i.idState ?? null;
        const key: Record<string, number | null> = { idState };
        let k = `${idState}`;
        if (swimlane) {
            key['assignedTo'] = i.assignedTo ?? null;
            k += `|${i.assignedTo ?? 'null'}`;
        }
        if (!map.has(k)) map.set(k, { key, items: [] });
        map.get(k)!.items.push(i);
    }
    return Array.from(map.values()).map(g => ({
        key: g.key,
        items: g.items,
        total: g.items.length,
        nextCursor: null
    }));
}

function buildService(issues: Issue[], states: IssueState[], usersArr: User[]): IssueKanbanService {
    const statesMap = new Map(states.map(s => [s.idState, s]));
    const usersMap = new Map(usersArr.map(u => [u.idUser, u]));
    const sevsMap = new Map<number, IssueSeverity>([[1, sev]]);

    const injector = Injector.create({
        providers: [
            { provide: SettingsStore, useValue: { kanbanPageSize: () => 20 } },
            {
                provide: IssueService,
                useValue: {
                    loadIssuesGrouped$: (_f: unknown, groupBy: string) =>
                        of({ groups: buildGroups(issues, groupBy) })
                }
            },
            {
                provide: IssueFilterStore,
                useValue: {
                    clear: () => {},
                    actualFilter$: of({ idProject: 1 }),
                    actualFilterChange$: of({ filter: { idProject: 1 }, refresh: false })
                }
            },
            { provide: SeverityStore, useValue: { severitiesMapByProject$: () => of(sevsMap) } },
            {
                provide: IssueTypeStore,
                useValue: { issueTypesMapByProject$: () => of(new Map()) }
            },
            { provide: StateStore, useValue: { statesMapByProject$: () => of(statesMap) } },
            { provide: ProjectMemberStore, useValue: { usersMap$: of(usersMap) } },
            { provide: SprintStore, useValue: { sprints$: of([]) } }
        ]
    });

    return runInInjectionContext(injector, () => new IssueKanbanService());
}

function firstEmit<T>(obs: { subscribe: (fn: (v: T) => void) => unknown }): T {
    let value!: T;
    obs.subscribe(v => (value = v));
    return value;
}

describe('IssueKanbanService — columns$', () => {
    it('builds a column per state with totals, tiles bucketed by state', () => {
        const svc = buildService(
            [makeIssue({ idIssue: 1, idState: 1 }), makeIssue({ idIssue: 2, idState: 2 })],
            [stateA, stateB],
            []
        );
        const columns = firstEmit<KanbanColumn[]>(svc.columns$);
        expect(columns).toHaveLength(2);
        expect(columns.find(c => c.state.idState === 1)?.total).toBe(1);
        expect(columns.find(c => c.state.idState === 2)?.tiles.length).toBe(1);
    });
});

describe('IssueKanbanService — swimlaneRows$', () => {
    it('unassigned row (user === undefined) is always first', () => {
        const svc = buildService(
            [makeIssue({ idIssue: 1, assignedTo: alice.idUser, idState: 1 })],
            [stateA, stateB],
            [alice]
        );
        const rows = firstEmit<SwimlaneRow[]>(svc.swimlaneRows$);
        expect(rows[0].user).toBeUndefined();
    });

    it('each distinct assignedToUser gets exactly one row', () => {
        const svc = buildService(
            [
                makeIssue({ idIssue: 1, assignedTo: alice.idUser, idState: 1 }),
                makeIssue({ idIssue: 2, assignedTo: bob.idUser, idState: 1 })
            ],
            [stateA],
            [alice, bob]
        );
        const rows = firstEmit<SwimlaneRow[]>(svc.swimlaneRows$);
        expect(rows.filter(r => r.user !== undefined).length).toBe(2);
    });

    it('tiles are placed in the correct cell (matching state + user)', () => {
        const svc = buildService(
            [makeIssue({ idIssue: 1, assignedTo: alice.idUser, idState: 2 })],
            [stateA, stateB],
            [alice]
        );
        const rows = firstEmit<SwimlaneRow[]>(svc.swimlaneRows$);
        const aliceRow = rows.find(r => r.user?.idUser === alice.idUser);
        expect(aliceRow?.cells.find(c => c.state.idState === 2)?.tiles.length).toBe(1);
        expect(aliceRow?.cells.find(c => c.state.idState === 1)?.tiles.length).toBe(0);
    });

    it('issues with no assignedToUser appear in the unassigned row', () => {
        const svc = buildService(
            [makeIssue({ idIssue: 1, assignedTo: undefined, idState: 1 })],
            [stateA],
            []
        );
        const rows = firstEmit<SwimlaneRow[]>(svc.swimlaneRows$);
        const unassigned = rows.find(r => r.user === undefined);
        expect(unassigned?.cells.find(c => c.state.idState === 1)?.tiles.length).toBe(1);
    });

    it('zero issues: returns rows with empty cells without error', () => {
        const svc = buildService([], [stateA, stateB], []);
        const rows = firstEmit<SwimlaneRow[]>(svc.swimlaneRows$);
        expect(rows.length).toBe(1); // only unassigned row
        rows[0].cells.forEach(c => expect(c.tiles.length).toBe(0));
    });
});

describe('IssueKanbanService — states$', () => {
    it('returns states in the same order as the state map iteration', () => {
        const svc = buildService([], [stateA, stateB], []);
        const states = firstEmit<IssueState[]>(svc.states$);
        expect(states[0].idState).toBe(1);
        expect(states[1].idState).toBe(2);
    });
});

describe('IssueKanbanService — applyRemoteIssue (live updates)', () => {
    function loadedService(issues: Issue[]): IssueKanbanService {
        const svc = buildService(issues, [stateA, stateB], [alice, bob]);
        // Subscribing loads the data and captures ctx
        svc.columns$.subscribe();
        svc.swimlaneRows$.subscribe();
        return svc;
    }

    it('moves the tile between columns and shifts both totals', () => {
        const svc = loadedService([
            makeIssue({ idIssue: 1, idState: 1 }),
            makeIssue({ idIssue: 2, idState: 1 })
        ]);
        const result = svc.applyRemoteIssue(makeIssue({ idIssue: 1, idState: 2 }), null, 'columns');
        expect(result).toBe('moved');

        const columns = firstEmit<KanbanColumn[]>(svc.columns$);
        const todo = columns.find(c => c.state.idState === 1)!;
        const done = columns.find(c => c.state.idState === 2)!;
        expect(todo.tiles.map(t => t.idIssue)).toEqual([2]);
        expect(todo.total).toBe(1);
        expect(done.tiles.map(t => t.idIssue)).toEqual([1]);
        expect(done.total).toBe(1);
    });

    it('same-column change updates the tile in place (own-drag echo → no move)', () => {
        const svc = loadedService([makeIssue({ idIssue: 1, idState: 1, title: 'old' })]);
        const result = svc.applyRemoteIssue(
            makeIssue({ idIssue: 1, idState: 1, title: 'renamed' }),
            null,
            'columns'
        );
        expect(result).toBe('updated');
        const columns = firstEmit<KanbanColumn[]>(svc.columns$);
        expect(columns.find(c => c.state.idState === 1)!.tiles[0].title).toBe('renamed');
    });

    it('issue leaving the sprint scope is removed from the board', () => {
        const svc = loadedService([makeIssue({ idIssue: 1, idState: 1 })]);
        const result = svc.applyRemoteIssue(
            makeIssue({ idIssue: 1, idState: 1, idSprint: 9 }),
            null,
            'columns'
        );
        expect(result).toBe('updated');
        const columns = firstEmit<KanbanColumn[]>(svc.columns$);
        expect(columns.find(c => c.state.idState === 1)!.tiles).toHaveLength(0);
        expect(columns.find(c => c.state.idState === 1)!.total).toBe(0);
    });

    it('unknown issue reports missing so the caller reloads', () => {
        const svc = loadedService([makeIssue({ idIssue: 1, idState: 1 })]);
        expect(svc.applyRemoteIssue(makeIssue({ idIssue: 99, idState: 2 }), null, 'columns')).toBe(
            'missing'
        );
    });

    it("verdict comes from the ACTIVE view — a stale hit in the inactive view can't mask a miss", () => {
        // Swimlane has the issue loaded, columns doesn't (only swimlane subscribed)
        const svc = buildService(
            [makeIssue({ idIssue: 1, idState: 1 })],
            [stateA, stateB],
            [alice]
        );
        svc.swimlaneRows$.subscribe();
        const result = svc.applyRemoteIssue(makeIssue({ idIssue: 1, idState: 2 }), null, 'columns');
        expect(result).toBe('missing');
    });

    it('assignee change moves the tile across swimlane rows', () => {
        const svc = loadedService([
            makeIssue({ idIssue: 1, idState: 1, assignedTo: alice.idUser }),
            makeIssue({ idIssue: 2, idState: 1, assignedTo: bob.idUser })
        ]);
        const result = svc.applyRemoteIssue(
            makeIssue({ idIssue: 1, idState: 1, assignedTo: bob.idUser }),
            null,
            'swimlane'
        );
        expect(result).toBe('moved');
        const rows = firstEmit<SwimlaneRow[]>(svc.swimlaneRows$);
        const aliceCell = rows
            .find(r => r.user?.idUser === alice.idUser)!
            .cells.find(c => c.state.idState === 1)!;
        const bobCell = rows
            .find(r => r.user?.idUser === bob.idUser)!
            .cells.find(c => c.state.idState === 1)!;
        expect(aliceCell.tiles).toHaveLength(0);
        expect(bobCell.tiles.map(t => t.idIssue)).toEqual([1, 2]);
        expect(bobCell.total).toBe(2);
    });
});

// A refresh() (e.g. after dragging a card between columns) must not throw away the
// "Load more" pages the user already fetched. Regression: refresh re-requested only the
// default page size, snapping every column back to page 1.
describe('IssueKanbanService — columns keep loaded pages on refresh', () => {
    function buildRefreshService(
        store: IssueFilterStore,
        groupedCalls: number[]
    ): IssueKanbanService {
        const statesMap = new Map([[stateA.idState, stateA]]);
        const usersMap = new Map<number, User>();
        const sevsMap = new Map<number, IssueSeverity>([[1, sev]]);
        const tile = (id: number) => makeIssue({ idIssue: id, idState: 1 });

        // Load more returns the next 20, exhausting the column.
        const pageObs = of<IssuesPage>({
            items: Array.from({ length: 20 }, (_, i) => tile(100 + i)),
            nextCursor: null,
            total: 40
        });

        const injector = Injector.create({
            providers: [
                { provide: SettingsStore, useValue: { kanbanPageSize: () => 20 } },
                {
                    provide: IssueService,
                    useValue: {
                        // Initial load returns a full page (20) with a cursor so "Load more" is available.
                        loadIssuesGrouped$: (_f: unknown, _groupBy: string, perGroup: number) => {
                            groupedCalls.push(perGroup);
                            const count = Math.min(perGroup, 40);
                            return of({
                                groups: [
                                    {
                                        key: { idState: 1 },
                                        items: Array.from({ length: count }, (_, i) => tile(i + 1)),
                                        total: 40,
                                        nextCursor: count < 40 ? 'more' : null
                                    }
                                ]
                            });
                        },
                        loadIssuesPage$: () => pageObs
                    }
                },
                {
                    provide: IssueFilterStore,
                    useValue: {
                        clear: () => {},
                        actualFilter$: of({ idProject: 1 }),
                        actualFilterChange$: store.actualFilterChange$
                    }
                },
                {
                    provide: SeverityStore,
                    useValue: { severitiesMapByProject$: () => of(sevsMap) }
                },
                {
                    provide: IssueTypeStore,
                    useValue: { issueTypesMapByProject$: () => of(new Map()) }
                },
                { provide: StateStore, useValue: { statesMapByProject$: () => of(statesMap) } },
                { provide: ProjectMemberStore, useValue: { usersMap$: of(usersMap) } },
                { provide: SprintStore, useValue: { sprints$: of([]) } }
            ]
        });
        return runInInjectionContext(injector, () => new IssueKanbanService());
    }

    it('re-requests the widest column extent on refresh, not just the page size', () => {
        const store = new IssueFilterStore();
        const groupedCalls: number[] = [];
        const svc = buildRefreshService(store, groupedCalls);

        let columns: KanbanColumn[] = [];
        svc.columns$.subscribe(c => (columns = c));

        store.setInitialFilter(initialFilter()); // grouped(perGroup=20) → 20 tiles + cursor
        const col = columns.find(c => c.state.idState === 1)!;
        svc.loadMoreColumn(col); // → 40 tiles loaded
        expect(columns.find(c => c.state.idState === 1)!.tiles.length).toBe(40);

        store.refresh();

        // Last grouped call must cover the loaded extent (40), not fall back to page size (20).
        expect(groupedCalls[groupedCalls.length - 1]).toBe(40);
        expect(columns.find(c => c.state.idState === 1)!.tiles.length).toBe(40);
    });
});
