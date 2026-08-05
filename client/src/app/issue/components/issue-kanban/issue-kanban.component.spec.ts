import { DestroyRef, Injector, runInInjectionContext } from '@angular/core';
import { EMPTY, of, throwError } from 'rxjs';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { TranslateService } from '@ngx-translate/core';
import { IssueKanbanComponent } from './issue-kanban.component';
import { IssueService } from '../../issue.service';
import { IssueKanbanService } from './service/issue-kanban.service';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { SavedViewApi } from 'src/app/project/api/saved-view.api.service';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { ProjectStore } from 'src/app/project/project.store';
import { SprintStore } from '../../store/sprint.store';
import { SprintApi } from '../../api/sprint.api.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { IssueState } from 'src/app/state/model/issue-state.model';
import { KanbanTile } from './entity/kanban-tile.entity';
import { KanbanColumn } from './entity/kanban-column.entity';
import { SwimlaneCell } from './entity/swimlane-cell.entity';
import { User } from 'src/app/auth/model/user.model';
import { CdkDragDrop } from '@angular/cdk/drag-drop';
import { Sprint } from '../../model/sprint.model';

// Node env has no localStorage; the component persists display settings there.
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
    } as KanbanTile;
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
}

function setup(sprints: Sprint[] = []): Harness {
    const updateIssue = vi.fn().mockReturnValue(of({} as never));
    const assignIssue = vi.fn().mockReturnValue(of(undefined));
    const refresh = vi.fn();
    const showError = vi.fn();
    const setSprint = vi.fn();
    const setInitialFilter = vi.fn();

    const injector = Injector.create({
        providers: [
            { provide: DestroyRef, useValue: { onDestroy: () => () => undefined } },
            { provide: IssueService, useValue: { updateIssue } },
            {
                provide: IssueKanbanService,
                useValue: { columns$: of([]), swimlaneRows$: of([]), states$: of([]) }
            },
            {
                provide: SprintStore,
                useValue: {
                    sprints$: of(sprints),
                    currentSprint$: of(undefined),
                    load: () => undefined,
                    create: () => undefined
                }
            },
            { provide: SprintApi, useValue: { assignIssue$: assignIssue } },
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
            { provide: NoticeService, useValue: { issue$: EMPTY } },
            { provide: TranslateService, useValue: { instant: (k: string) => k } },
            { provide: ProjectStore, useValue: { project$: of({ idProject: 1 }) } },
            // The board reads a staged saved view on mount and drives its layout from
            // activeView; a stubbed api keeps the real store buildable here.
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
        savedViewStore: injector.get(SavedViewStore)
    };
}

// Handlers are protected — reach them through a narrow cast.
type Handlers = {
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
};
const handlers = (c: IssueKanbanComponent): Handlers => c as unknown as Handlers;

function makeSprint(overrides: Partial<Sprint> & { idSprint: number }): Sprint {
    return {
        idProject: 1,
        name: `Sprint ${overrides.idSprint}`,
        startAt: '2020-01-01T00:00:00Z',
        endAt: '2099-01-01T00:00:00Z',
        state: 'planned',
        ...overrides
    };
}

// One open sprint whose window contains "now" (= current), plus two closed ones.
const openSprint = makeSprint({ idSprint: 1 });
const closedOld = makeSprint({
    idSprint: 2,
    state: 'closed',
    startAt: '2026-01-01T00:00:00Z',
    endAt: '2026-01-15T00:00:00Z'
});
const closedNew = makeSprint({
    idSprint: 3,
    state: 'closed',
    startAt: '2026-02-01T00:00:00Z',
    endAt: '2026-02-15T00:00:00Z'
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

        // Notifying is ErrorInterceptor's job; the board only has to reconcile.
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

        // Notifying is ErrorInterceptor's job; the board only has to reconcile.
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
