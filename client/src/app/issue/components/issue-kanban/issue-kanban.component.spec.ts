import { DestroyRef, Injector, runInInjectionContext } from '@angular/core';
import { SprintState } from '../../constants/sprint-state.enum';
import { EMPTY, NEVER, Observable, of, Subject, throwError } from 'rxjs';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { IssueKanbanComponent } from './issue-kanban.component';
import { IssueService } from '../../issue.service';
import { IssueKanbanService } from './service/issue-kanban.service';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { SavedViewApi } from 'src/app/project/api/saved-view.api.service';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { ProjectStore } from 'src/app/project/project.store';
import { SprintStore } from '../../store/sprint.store';
import { SprintAnalyticsStore, STATS_DEBOUNCE_MS } from '../../store/sprint-analytics.store';
import { SprintApi } from '../../api/sprint.api.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { KanbanTile } from './entity/kanban-tile.entity';
import { KanbanColumn } from './entity/kanban-column.entity';
import { SwimlaneCell } from './entity/swimlane-cell.entity';
import { User } from 'src/app/auth/model/user.model';
import { CdkDragDrop } from '@angular/cdk/drag-drop';
import { Sprint } from '../../model/sprint.model';
import { SprintVelocity } from '../../model/sprint-velocity.model';
import { SprintStats } from '../../model/sprint-stats.model';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { NoticeSubject } from 'src/app/shared/notice/constant/notice-subject.enum';
import { Issue } from '../../model/issue.model';

const storage = new Map<string, string>();
vi.stubGlobal('localStorage', {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => storage.set(key, value),
    removeItem: (key: string) => storage.delete(key)
});

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

const alice: User = {
    idUser: 10,
    name: 'Alice',
    email: 'a@a.com',
    colorAvatarBg: '#aaa'
};
const bob: User = {
    idUser: 20,
    name: 'Bob',
    email: 'b@b.com',
    colorAvatarBg: '#bbb'
};

function makeTile(overrides: Partial<KanbanTile> = {}): KanbanTile {
    return {
        idIssue: 1,
        idProject: 1,
        idState: 1,
        idSeverity: 1,
        title: 'test',
        description: '',
        tracked: 0,
        state: stateA,
        severity: undefined,
        createUser: undefined,
        updateUser: undefined,
        assignedToUser: alice,
        assignedTo: alice.idUser,
        ...overrides
    };
}

function issueNotice(idProject: number, idIssue: number): Notice<Issue> {
    return {
        subject: NoticeSubject.Issue,
        action: NoticeAction.Create,
        payload: {
            idIssue,
            idIssuePublic: idIssue,
            idProject,
            idState: null,
            idSeverity: null,
            title: 'T',
            description: '',
            tracked: 0
        }
    };
}

function makeColumn(state: IssueState, tiles: KanbanTile[]): KanbanColumn {
    return { state, tiles, total: tiles.length, cursor: null, loading: false };
}

function makeColumnDropEvent(
    fromColumn: KanbanColumn,
    toColumn: KanbanColumn,
    previousIndex = 0
): CdkDragDrop<KanbanColumn> {
    return {
        previousContainer: { data: fromColumn } as any,
        container: { data: toColumn } as any,
        previousIndex,
        currentIndex: 0,
        item: {} as any,
        isPointerOverContainer: true,
        distance: { x: 0, y: 0 },
        dropPoint: { x: 0, y: 0 },
        event: {} as any
    };
}

function makeCellDropEvent(
    tile: KanbanTile,
    fromTiles: KanbanTile[],
    toCell: SwimlaneCell,
    previousIndex = 0
): CdkDragDrop<SwimlaneCell> {
    const fromCell: SwimlaneCell = { state: stateA, user: alice, tiles: fromTiles };
    return {
        previousContainer: { data: fromCell } as any,
        container: { data: toCell } as any,
        previousIndex,
        currentIndex: 0,
        item: {} as any,
        isPointerOverContainer: true,
        distance: { x: 0, y: 0 },
        dropPoint: { x: 0, y: 0 },
        event: {} as any
    };
}

interface Harness {
    component: IssueKanbanComponent;
    updateIssue: ReturnType<typeof vi.fn>;
    assignIssue: ReturnType<typeof vi.fn>;
    refresh: ReturnType<typeof vi.fn>;
    showError: ReturnType<typeof vi.fn>;
    setSprint: ReturnType<typeof vi.fn>;
    setInitialFilter: ReturnType<typeof vi.fn>;
    savedViewStore: SavedViewStore;
    stats: ReturnType<typeof vi.fn>;
    backlogStats: ReturnType<typeof vi.fn>;
    velocity: ReturnType<typeof vi.fn>;
    injector: Injector;
}

function makeStats(overrides: Partial<SprintStats> = {}): SprintStats {
    return {
        totalPoints: 0,
        donePoints: 0,
        startPoints: 0,
        progressPoints: 0,
        totalIssues: 0,
        doneIssues: 0,
        startIssues: 0,
        progressIssues: 0,
        pointedIssues: 0,
        ...overrides
    };
}

function setup(
    sprints: Sprint[] = [],
    project: { idProject: number } | null = { idProject: 1 },
    issueNotices: Observable<Notice<Issue>> = EMPTY
): Harness {
    const updateIssue = vi.fn().mockReturnValue(of({} as never));
    const assignIssue = vi.fn().mockReturnValue(of(undefined));
    const refresh = vi.fn();
    const showError = vi.fn();
    const setSprint = vi.fn();
    const setInitialFilter = vi.fn();
    const stats = vi.fn().mockReturnValue(of(makeStats()));
    const backlogStats = vi.fn().mockReturnValue(of(makeStats()));
    const velocity = vi.fn().mockReturnValue(of([]));

    const injector = Injector.create({
        providers: [
            { provide: DestroyRef, useValue: { onDestroy: () => () => undefined } },
            { provide: IssueService, useValue: { updateIssue } },
            {
                provide: IssueKanbanService,
                useValue: { columns$: of([]), swimlaneRows$: of([]), states$: of([]) }
            },
            SprintStore,
            SprintAnalyticsStore,
            {
                provide: SprintApi,
                useValue: {
                    assignIssue$: assignIssue,
                    close$: () => of({ moved: 0 }),
                    loadByProject$: () => of(sprints),
                    create$: () => of(sprints[0] ?? null),
                    edit$: () => of(sprints[0] ?? null),
                    delete$: () => of(undefined),
                    loadSprintStats$: stats,
                    loadBacklogStats$: backlogStats,
                    loadVelocity$: velocity
                }
            },
            {
                provide: IssueFilterStore,
                useValue: {
                    showFilter$: of(false),
                    initialFilter$: EMPTY,
                    setInitialFilter,
                    setSprint,
                    refresh
                }
            },
            {
                provide: IssueToolbarService,
                useValue: {
                    register: () => undefined,
                    clear: () => undefined
                }
            },
            { provide: ToastNotificationService, useValue: { showError } },
            { provide: NoticeService, useValue: { issue$: issueNotices } },
            { provide: I18nService, useValue: { instant: (k: string) => k } },
            {
                provide: ProjectStore,
                useValue: { project$: project === null ? NEVER : of(project) }
            },
            { provide: SavedViewApi, useValue: { loadByProject$: () => of([]) } },
            SavedViewStore
        ]
    });

    const component = runInInjectionContext(injector, () => {
        const created = new IssueKanbanComponent();
        created.ngOnInit();
        return created;
    });
    return {
        component,
        updateIssue,
        assignIssue,
        refresh,
        showError,
        setSprint,
        setInitialFilter,
        savedViewStore: injector.get(SavedViewStore),
        stats,
        backlogStats,
        velocity,
        injector
    };
}

interface Handlers {
    onStateChange(evt: CdkDragDrop<KanbanColumn>): void;
    onSwimlaneCardDrop(evt: CdkDragDrop<SwimlaneCell>): void;
    onTabTaskDropped(payload: { idSprint: number | null; event: CdkDragDrop<unknown> }): void;
    sprintTabs(): {
        idSprint: number | null;
        label: string;
        isCurrent: boolean;
        isClosed: boolean;
        listId: string;
    }[];
    sprintTabListIds(): string[];
    selectedIdSprint(): number | null;
    showClosedSprints(): boolean;
    onShowClosedSprintsChange(value: boolean): void;
    onSprintChange(idSprint: number | null): void;
    onEditSprint(idSprint: number): void;
    onSprintDeleted(): void;
}
const handlers = (c: IssueKanbanComponent): Handlers => c as unknown as Handlers;

interface Analytics {
    stats(): SprintStats | null;
    velocities(): SprintVelocity[];
    onRollOver(): void;
}
const analytics = (c: IssueKanbanComponent): Analytics => c as unknown as Analytics;

function setupWithNotices(notices: Observable<Notice<Issue>>): Harness {
    return setup([], { idProject: 1 }, notices);
}

function makeSprint(overrides: Partial<Sprint> & { idSprint: number }): Sprint {
    return {
        idProject: 1,
        name: `Sprint ${overrides.idSprint}`,
        startAt: '2020-01-01T00:00:00Z',
        endAt: '2099-01-01T00:00:00Z',
        state: SprintState.Planned,
        ...overrides
    };
}

const openSprint = makeSprint({ idSprint: 1 });
const closedOld = makeSprint({
    idSprint: 2,
    state: SprintState.Closed,
    startAt: '2026-01-01T00:00:00Z',
    endAt: '2026-01-15T00:00:00Z'
});
const secondSprint = makeSprint({ idSprint: 4 });
const closedNew = makeSprint({
    idSprint: 3,
    state: SprintState.Closed,
    startAt: '2026-02-01T00:00:00Z',
    endAt: '2026-02-15T00:00:00Z'
});

describe('IssueKanbanComponent — tab order', () => {
    beforeEach(() => storage.clear());

    it('orders open sprints by start date, not by which one is current', () => {
        const overdue = makeSprint({
            idSprint: 3,
            startAt: '2026-07-26T00:00:00Z',
            endAt: '2026-08-10T00:00:00Z'
        });
        const active = makeSprint({
            idSprint: 4,
            startAt: '2026-08-10T00:00:00Z',
            endAt: '2026-08-16T00:00:00Z'
        });
        const future = makeSprint({
            idSprint: 5,
            startAt: '2026-08-17T00:00:00Z',
            endAt: '2026-08-23T00:00:00Z'
        });

        const tabs = handlers(setup([active, overdue, future]).component).sprintTabs();

        expect(tabs.map(t => t.idSprint)).toEqual([null, 3, 4, 5]);
    });

    it('still marks the current cycle, wherever it sits in the order', () => {
        const past = makeSprint({
            idSprint: 3,
            startAt: '2019-01-01T00:00:00Z',
            endAt: '2019-01-15T00:00:00Z'
        });
        const tabs = handlers(setup([openSprint, past]).component).sprintTabs();

        expect(tabs.map(t => t.idSprint)).toEqual([null, 3, 1]);
        expect(tabs.find(t => t.isCurrent)?.idSprint).toBe(openSprint.idSprint);
    });
});

describe('IssueKanbanComponent — closed sprints display setting', () => {
    beforeEach(() => storage.clear());

    it('hides closed sprints from the tab strip by default', () => {
        const h = setup([openSprint, closedOld, closedNew]);
        const tabs = handlers(h.component).sprintTabs();
        expect(tabs.map(t => t.idSprint)).toEqual([null, 1]);
    });

    it('appends closed sprints after open ones, most recently ended first', () => {
        const h = setup([openSprint, closedOld, closedNew]);
        handlers(h.component).onShowClosedSprintsChange(true);
        const tabs = handlers(h.component).sprintTabs();
        expect(tabs.map(t => t.idSprint)).toEqual([null, 1, 3, 2]);
        expect(tabs.map(t => t.isClosed)).toEqual([false, false, true, true]);
    });

    it('excludes closed tabs from the drop-target list ids', () => {
        const h = setup([openSprint, closedOld, closedNew]);
        handlers(h.component).onShowClosedSprintsChange(true);
        expect(handlers(h.component).sprintTabListIds()).toEqual([
            'sprint-tab-backlog',
            'sprint-tab-1'
        ]);
    });

    it('persists the setting to localStorage and restores it on construction', () => {
        const h = setup([openSprint, closedOld]);
        handlers(h.component).onShowClosedSprintsChange(true);
        expect(storage.get('issue-kanban-show-closed-sprints')).toBe('true');

        const h2 = setup([openSprint, closedOld]);
        expect(handlers(h2.component).showClosedSprints()).toBe(true);
        expect(
            handlers(h2.component)
                .sprintTabs()
                .some(t => t.isClosed)
        ).toBe(true);
    });

    it('resets the board scope to the default when hiding a selected closed sprint', () => {
        const h = setup([openSprint, closedOld, closedNew]);
        handlers(h.component).onShowClosedSprintsChange(true);
        handlers(h.component).onSprintChange(closedNew.idSprint);
        h.setSprint.mockClear();

        handlers(h.component).onShowClosedSprintsChange(false);

        expect(handlers(h.component).selectedIdSprint()).toBe(openSprint.idSprint);
        expect(h.setSprint).toHaveBeenCalledWith(openSprint.idSprint);
    });

    it('keeps the scope when hiding closed sprints while an open sprint is selected', () => {
        const h = setup([openSprint, closedOld]);
        handlers(h.component).onShowClosedSprintsChange(true);
        handlers(h.component).onSprintChange(openSprint.idSprint);
        h.setSprint.mockClear();

        handlers(h.component).onShowClosedSprintsChange(false);

        expect(handlers(h.component).selectedIdSprint()).toBe(openSprint.idSprint);
        expect(h.setSprint).not.toHaveBeenCalled();
    });
});

describe('IssueKanbanComponent — onStateChange (columns)', () => {
    it('calls updateIssue with the new idState and state', () => {
        const h = setup();
        const tile = makeTile({ idState: 1, state: stateA });

        handlers(h.component).onStateChange(
            makeColumnDropEvent(makeColumn(stateA, [tile]), makeColumn(stateB, []))
        );

        expect(h.updateIssue).toHaveBeenCalledTimes(1);
        expect(h.updateIssue).toHaveBeenCalledWith(
            expect.objectContaining({ idState: stateB.idState, state: stateB })
        );
    });

    it('on success: does not refresh the board nor show a toast', () => {
        const h = setup();
        const tile = makeTile({ idState: 1, state: stateA });

        handlers(h.component).onStateChange(
            makeColumnDropEvent(makeColumn(stateA, [tile]), makeColumn(stateB, []))
        );

        expect(h.refresh).not.toHaveBeenCalled();
        expect(h.showError).not.toHaveBeenCalled();
    });

    it('on error: refreshes the board', () => {
        const h = setup();
        h.updateIssue.mockReturnValue(throwError(() => new Error('403')));
        const tile = makeTile({ idState: 1, state: stateA });

        handlers(h.component).onStateChange(
            makeColumnDropEvent(makeColumn(stateA, [tile]), makeColumn(stateB, []))
        );

        expect(h.refresh).toHaveBeenCalledTimes(1);
    });
});

describe('IssueKanbanComponent — onSwimlaneCardDrop (swimlane)', () => {
    it('state change: calls updateIssue with the new idState and state', () => {
        const h = setup();
        const tile = makeTile({ idState: 1, state: stateA });
        const toCell: SwimlaneCell = { state: stateB, user: alice, tiles: [] };

        handlers(h.component).onSwimlaneCardDrop(makeCellDropEvent(tile, [tile], toCell));

        expect(h.updateIssue).toHaveBeenCalledTimes(1);
        expect(h.updateIssue).toHaveBeenCalledWith(
            expect.objectContaining({ idState: stateB.idState, state: stateB })
        );
    });

    it('user change: calls updateIssue with the new assignedTo and assignedToUser', () => {
        const h = setup();
        const tile = makeTile({ assignedTo: alice.idUser, assignedToUser: alice });
        const toCell: SwimlaneCell = { state: stateA, user: bob, tiles: [] };

        handlers(h.component).onSwimlaneCardDrop(makeCellDropEvent(tile, [tile], toCell));

        expect(h.updateIssue).toHaveBeenCalledTimes(1);
        expect(h.updateIssue).toHaveBeenCalledWith(
            expect.objectContaining({ assignedTo: bob.idUser, assignedToUser: bob })
        );
    });

    it('no-op drop (same state and user): does not call updateIssue', () => {
        const h = setup();
        const tile = makeTile({ idState: 1, assignedTo: alice.idUser });
        const toCell: SwimlaneCell = { state: stateA, user: alice, tiles: [] };

        handlers(h.component).onSwimlaneCardDrop(makeCellDropEvent(tile, [tile], toCell));

        expect(h.updateIssue).not.toHaveBeenCalled();
    });

    it('on success: does not refresh the board nor show a toast', () => {
        const h = setup();
        const tile = makeTile({ idState: 1, state: stateA });
        const toCell: SwimlaneCell = { state: stateB, user: alice, tiles: [] };

        handlers(h.component).onSwimlaneCardDrop(makeCellDropEvent(tile, [tile], toCell));

        expect(h.refresh).not.toHaveBeenCalled();
        expect(h.showError).not.toHaveBeenCalled();
    });

    it('on error: refreshes the board', () => {
        const h = setup();
        h.updateIssue.mockReturnValue(throwError(() => new Error('409')));
        const tile = makeTile({ idState: 1, state: stateA });
        const toCell: SwimlaneCell = { state: stateB, user: alice, tiles: [] };

        handlers(h.component).onSwimlaneCardDrop(makeCellDropEvent(tile, [tile], toCell));

        expect(h.refresh).toHaveBeenCalledTimes(1);
    });
});

describe('IssueKanbanComponent — onTabTaskDropped (drag onto sprint tab)', () => {
    function tabDrop(
        source: KanbanColumn,
        idSprint: number | null,
        previousIndex = 0
    ): {
        idSprint: number | null;
        event: CdkDragDrop<unknown>;
    } {
        return {
            idSprint,
            event: {
                previousContainer: { data: source },
                previousIndex
            } as unknown as CdkDragDrop<unknown>
        };
    }

    it('assigns the dragged task to the target sprint', () => {
        const h = setup();
        const tile = makeTile({ idIssue: 5, idIssuePublic: 42, idProject: 1, idSprint: null });

        handlers(h.component).onTabTaskDropped(tabDrop(makeColumn(stateA, [tile]), 7));

        expect(h.assignIssue).toHaveBeenCalledWith(1, 42, 7);
    });

    it('removes the task from the source list so the reparented node is reclaimed (no top-left orphan)', () => {
        const h = setup();
        const tile = makeTile({ idIssue: 5, idIssuePublic: 42, idProject: 1, idSprint: null });
        const source = makeColumn(stateA, [tile]);

        handlers(h.component).onTabTaskDropped(tabDrop(source, 7));

        expect(source.tiles).toHaveLength(0);
        expect(source.total).toBe(0);
    });

    it('no-op drop (already in that sprint): refreshes to restore the DOM and does not assign', () => {
        const h = setup();
        const tile = makeTile({ idIssue: 5, idIssuePublic: 42, idProject: 1, idSprint: 7 });
        const source = makeColumn(stateA, [tile]);

        handlers(h.component).onTabTaskDropped(tabDrop(source, 7));

        expect(h.assignIssue).not.toHaveBeenCalled();
        expect(source.tiles).toHaveLength(1);
        expect(h.refresh).toHaveBeenCalledTimes(1);
    });

    it('on success: refreshes the board to reconcile', () => {
        const h = setup();
        const tile = makeTile({ idIssue: 5, idIssuePublic: 42, idProject: 1, idSprint: null });

        handlers(h.component).onTabTaskDropped(tabDrop(makeColumn(stateA, [tile]), 7));

        expect(h.refresh).toHaveBeenCalledTimes(1);
        expect(h.showError).not.toHaveBeenCalled();
    });

    it('on error: refreshes the board', () => {
        const h = setup();
        h.assignIssue.mockReturnValue(throwError(() => new Error('403')));
        const tile = makeTile({ idIssue: 5, idIssuePublic: 42, idProject: 1, idSprint: null });

        handlers(h.component).onTabTaskDropped(tabDrop(makeColumn(stateA, [tile]), 7));

        expect(h.refresh).toHaveBeenCalledTimes(1);
    });
});

describe('IssueKanbanComponent — saved view reset', () => {
    it('mounts without a sprint scope in the default filter', () => {
        const h = setup([openSprint]);

        expect(h.setInitialFilter).toHaveBeenCalledTimes(1);
        expect(h.setInitialFilter.mock.calls[0][0]).not.toHaveProperty('idSprint');
    });

    it('keeps the selected sprint when the applied view is cleared', () => {
        const h = setup([openSprint]);
        handlers(h.component).onSprintChange(openSprint.idSprint);
        h.setInitialFilter.mockClear();

        h.savedViewStore.sendFilterResetSignal();

        expect(h.setInitialFilter).toHaveBeenCalledWith(
            expect.objectContaining({ idSprint: openSprint.idSprint, sprintUnset: false })
        );
    });

    it('scopes the reset to the backlog when the backlog tab is selected', () => {
        const h = setup([openSprint]);
        handlers(h.component).onSprintChange(null);
        h.setInitialFilter.mockClear();

        h.savedViewStore.sendFilterResetSignal();

        expect(h.setInitialFilter).toHaveBeenCalledWith(
            expect.objectContaining({ idSprint: null, sprintUnset: true })
        );
    });
});

describe('IssueKanbanComponent — sprint analytics', () => {
    beforeEach(() => {
        storage.clear();
        vi.useFakeTimers();
    });

    afterEach(() => vi.useRealTimers());

    const settle = (): void => {
        vi.advanceTimersByTime(STATS_DEBOUNCE_MS);
    };

    it('loads stats for the auto-selected cycle and velocity once the project resolves', () => {
        const h = setup([openSprint]);

        expect(h.velocity).toHaveBeenCalledWith(1);
        expect(h.stats).toHaveBeenCalledWith(openSprint.idSprint);
    });

    it('loads backlog stats when the project has no open cycle to select', () => {
        const h = setup([]);
        expect(h.backlogStats).toHaveBeenCalledWith(1);
    });

    it('requests nothing while no project is loaded', () => {
        const h = setup([], null);
        expect(h.stats).not.toHaveBeenCalled();
        expect(h.backlogStats).not.toHaveBeenCalled();
        expect(h.velocity).not.toHaveBeenCalled();
    });

    it('switches between sprint stats and backlog stats with the tab, without waiting', () => {
        const h = setup([openSprint]);
        h.backlogStats.mockClear();

        handlers(h.component).onSprintChange(openSprint.idSprint);
        expect(h.stats).toHaveBeenCalledWith(openSprint.idSprint);

        handlers(h.component).onSprintChange(null);
        expect(h.backlogStats).toHaveBeenCalledWith(1);
    });

    it('keeps only the last response when the tab is switched fast', () => {
        const first = new Subject<SprintStats>();
        const second = new Subject<SprintStats>();
        const h = setup([openSprint, secondSprint]);
        h.stats.mockReturnValueOnce(first).mockReturnValueOnce(second);

        handlers(h.component).onSprintChange(openSprint.idSprint);
        handlers(h.component).onSprintChange(secondSprint.idSprint);

        first.next(makeStats({ totalPoints: 111 }));
        second.next(makeStats({ totalPoints: 222 }));

        expect(analytics(h.component).stats()?.totalPoints).toBe(222);
    });

    it('re-requests stats for an issue notice from this project only', () => {
        const notices = new Subject<Notice<Issue>>();
        const h = setupWithNotices(notices);
        h.backlogStats.mockClear();

        notices.next(issueNotice(2, 5));
        settle();
        expect(h.backlogStats).not.toHaveBeenCalled();

        notices.next(issueNotice(1, 5));
        settle();
        expect(h.backlogStats).toHaveBeenCalledWith(1);
    });

    it('clears the previous cycle stats when the tab changes, so no stale numbers show', () => {
        const pending = new Subject<SprintStats>();
        const h = setup([openSprint, secondSprint]);
        h.stats.mockReturnValue(of(makeStats({ totalPoints: 21 })));
        handlers(h.component).onSprintChange(openSprint.idSprint);
        expect(analytics(h.component).stats()?.totalPoints).toBe(21);

        h.stats.mockReturnValue(pending);
        handlers(h.component).onSprintChange(secondSprint.idSprint);
        expect(analytics(h.component).stats()).toBeNull();

        pending.next(makeStats({ totalPoints: 8 }));
        expect(analytics(h.component).stats()?.totalPoints).toBe(8);
    });

    it('keeps the current stats visible while a teammate notice refreshes them', () => {
        const notices = new Subject<Notice<Issue>>();
        const pending = new Subject<SprintStats>();
        const h = setupWithNotices(notices);
        h.backlogStats.mockReturnValue(of(makeStats({ totalIssues: 4 })));
        handlers(h.component).onSprintChange(null);
        expect(analytics(h.component).stats()?.totalIssues).toBe(4);

        h.backlogStats.mockReturnValue(pending);
        notices.next(issueNotice(1, 5));
        settle();

        expect(analytics(h.component).stats()?.totalIssues).toBe(4);
        pending.next(makeStats({ totalIssues: 5 }));
        expect(analytics(h.component).stats()?.totalIssues).toBe(5);
    });

    it('answers a burst of teammate notices with one stats request', () => {
        const notices = new Subject<Notice<Issue>>();
        const h = setupWithNotices(notices);
        h.backlogStats.mockClear();

        for (let i = 0; i < 20; i++) {
            notices.next(issueNotice(1, i));
        }
        settle();

        expect(h.backlogStats).toHaveBeenCalledTimes(1);
    });

    it('falls back to the backlog when the selected cycle is deleted', () => {
        const h = setup([openSprint]);
        handlers(h.component).onSprintChange(openSprint.idSprint);
        h.backlogStats.mockClear();

        handlers(h.component).onEditSprint(openSprint.idSprint);
        handlers(h.component).onSprintDeleted();

        expect(handlers(h.component).selectedIdSprint()).toBeNull();
        expect(h.backlogStats).toHaveBeenCalledWith(1);
    });

    it('leaves the scope alone when a cycle other than the selected one is deleted', () => {
        const h = setup([openSprint, secondSprint]);
        handlers(h.component).onSprintChange(openSprint.idSprint);

        handlers(h.component).onEditSprint(secondSprint.idSprint);
        handlers(h.component).onSprintDeleted();

        expect(handlers(h.component).selectedIdSprint()).toBe(openSprint.idSprint);
    });

    it('asks for the backlog once, not twice, after rolling a cycle over', () => {
        const h = setup([openSprint]);
        handlers(h.component).onSprintChange(openSprint.idSprint);
        h.backlogStats.mockClear();

        analytics(h.component).onRollOver();

        expect(h.backlogStats).toHaveBeenCalledTimes(1);
    });

    it('re-requests velocity after closing a sprint', () => {
        const h = setup([openSprint]);
        handlers(h.component).onSprintChange(openSprint.idSprint);
        h.velocity.mockClear();

        analytics(h.component).onRollOver();

        expect(h.velocity).toHaveBeenCalledWith(1);
    });

    it('keeps the loaded velocity list for the strip to average', () => {
        const h = setup([openSprint]);
        h.velocity.mockReturnValue(
            of([
                {
                    idSprint: 1,
                    name: 'a',
                    endAt: '',
                    donePoints: 100,
                    doneIssues: 50,
                    frozen: false
                },
                { idSprint: 2, name: 'b', endAt: '', donePoints: 2, doneIssues: 1, frozen: false },
                { idSprint: 3, name: 'c', endAt: '', donePoints: 4, doneIssues: 2, frozen: false },
                { idSprint: 4, name: 'd', endAt: '', donePoints: 6, doneIssues: 3, frozen: false },
                { idSprint: 5, name: 'e', endAt: '', donePoints: 8, doneIssues: 4, frozen: false },
                { idSprint: 6, name: 'f', endAt: '', donePoints: 10, doneIssues: 5, frozen: false }
            ])
        );
        analytics(h.component).onRollOver();

        expect(
            analytics(h.component)
                .velocities()
                .map(v => v.donePoints)
        ).toEqual([100, 2, 4, 6, 8, 10]);
    });
});
