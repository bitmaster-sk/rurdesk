// @vitest-environment jsdom
import { DestroyRef, Injector, runInInjectionContext, signal } from '@angular/core';
import {
    DefaultUrlSerializer,
    Event,
    NavigationEnd,
    NavigationStart,
    Router,
    RoutesRecognized,
    UrlTree
} from '@angular/router';
import { Subject } from 'rxjs';
import { SavedView } from 'src/app/project/model/saved-view.model';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { IssueViewMode } from '../constants/issue-view-modes.enum';
import { IssuesFilter } from '../components/filter/issue-filter.entity';
import { IssueFilterStore } from '../components/filter/issue-filter.store';
import { IssueListStateStore } from './issue-list-state.store';

const TABLE = '/project/1/issue/view/table';
const KANBAN = '/project/1/issue/view/kanban';
const DETAIL = '/project/1/issue/42';

class FakeRouter {
    public readonly events = new Subject<Event>();
    public url: string;
    private readonly serializer = new DefaultUrlSerializer();
    private idNavigation = 0;

    public constructor(url: string) {
        this.url = url;
    }

    public currentNavigation(): null {
        return null;
    }

    public parseUrl(url: string): UrlTree {
        return this.serializer.parse(url);
    }

    public serializeUrl(tree: UrlTree): string {
        return this.serializer.serialize(tree);
    }

    public start(url: string, trigger: 'imperative' | 'popstate' = 'imperative'): void {
        this.events.next(new NavigationStart(++this.idNavigation, url, trigger));
    }

    public redirect(url: string): void {
        this.events.next(new RoutesRecognized(this.idNavigation, url, url, undefined as never));
    }

    public end(url: string): void {
        this.url = url;
        this.events.next(new NavigationEnd(this.idNavigation, url, url));
    }

    public go(url: string, trigger: 'imperative' | 'popstate' = 'imperative'): void {
        this.start(url, trigger);
        this.end(url);
    }
}

interface Harness {
    store: IssueListStateStore;
    filterStore: IssueFilterStore;
    router: FakeRouter;
}

const savedView: SavedView = {
    idSavedView: 7,
    idProject: 1,
    name: 'Open bugs',
    viewType: IssueViewMode.TABLE,
    config: { v: 1, idsState: [3], orderColumn: 'title', orderDirection: 'asc' },
    isShared: false,
    createBy: 1,
    updateAt: '2026-08-01T00:00:00Z'
};

function setup(url: string, views: SavedView[] = [savedView]): Harness {
    const router = new FakeRouter(url);
    const filterStore = new IssueFilterStore();
    const injector = Injector.create({
        providers: [
            { provide: DestroyRef, useValue: { onDestroy: () => () => undefined } },
            { provide: Router, useValue: router },
            { provide: IssueFilterStore, useValue: filterStore },
            { provide: SavedViewStore, useValue: { views: signal(views) } }
        ]
    });
    const store = runInInjectionContext(injector, () => new IssueListStateStore());
    return { store, filterStore, router };
}

function baseFilter(idProject = 1): IssuesFilter {
    return { idProject, orderColumn: 'idIssue', orderDirection: 'desc' };
}

describe('IssueListStateStore', () => {
    beforeEach(() => localStorage.clear());

    describe('filter', () => {
        it('keeps an edited filter across a page reload', () => {
            const first = setup(TABLE);
            first.filterStore.setInitialFilter(baseFilter());
            first.filterStore.setFilter({
                title: 'login',
                createAtFrom: new Date('2026-09-01T00:00:00.000Z')
            });

            const reloaded = setup(TABLE);

            const restored = reloaded.store.restoreFilter();
            expect(restored?.title).toBe('login');
            expect(restored?.createAtFrom).toEqual(new Date('2026-09-01T00:00:00.000Z'));
        });

        it('does not treat the defaults a view installs as a user choice', () => {
            const { store, filterStore } = setup(TABLE);

            filterStore.setInitialFilter(baseFilter());

            expect(store.restoreFilter()).toBeNull();
        });

        it('keeps each view mode on its own filter', () => {
            const { store, filterStore, router } = setup(TABLE);
            filterStore.setInitialFilter(baseFilter());
            filterStore.setFilter({ title: 'table only' });

            router.go(KANBAN);

            expect(store.restoreFilter()).toBeNull();
            router.go(TABLE);
            expect(store.restoreFilter()?.title).toBe('table only');
        });

        it('keeps each project on its own filter', () => {
            const { store, filterStore, router } = setup(TABLE);
            filterStore.setInitialFilter(baseFilter());
            filterStore.setFilter({ title: 'project one' });

            router.go('/project/2/issue/view/table');

            expect(store.restoreFilter()).toBeNull();
        });

        it('does not remember the sprint, so the board picks the current one on its own', () => {
            const first = setup(KANBAN);
            first.filterStore.setInitialFilter(baseFilter());
            first.filterStore.setSprint(4);

            const restored = setup(KANBAN).store.restoreFilter();

            expect(restored?.idSprint).toBeUndefined();
            expect(restored?.sprintUnset).toBeUndefined();
        });

        it('falls back to the saved view itself when it was not edited', () => {
            const { store } = setup(`${TABLE}?view=7`);

            expect(store.restoreFilter()).toEqual({
                idProject: 1,
                idsState: [3],
                orderColumn: 'title',
                orderDirection: 'asc'
            });
        });

        it('ignores a saved view from another project', () => {
            const { store } = setup('/project/2/issue/view/table?view=7');

            expect(store.restoreFilter()).toBeNull();
        });

        it('shows a saved view as saved and keeps its edits out of the ad-hoc filter', () => {
            const { store, filterStore, router } = setup(`${TABLE}?view=7`);
            filterStore.setInitialFilter(baseFilter());
            filterStore.setFilter({ title: 'tweaked view' });

            expect(store.restoreFilter()?.title).toBeUndefined();
            expect(store.restoreFilter()?.idsState).toEqual([3]);
            router.go(TABLE);
            expect(store.restoreFilter()).toBeNull();
        });

        it('shows the defaults for a saved view that is not loaded yet', () => {
            expect(setup(`${TABLE}?view=7`, []).store.restoreFilter()).toBeNull();
        });
        it('forgets the stored filter on request', () => {
            const { store, filterStore } = setup(TABLE);
            filterStore.setInitialFilter(baseFilter());
            filterStore.setFilter({ title: 'login' });

            store.forgetFilter();

            expect(store.restoreFilter()).toBeNull();
            expect(setup(TABLE).store.restoreFilter()).toBeNull();
        });

        it('finds the remembered filter when the list is reached through a redirect', () => {
            const first = setup(TABLE);
            first.filterStore.setInitialFilter(baseFilter());
            first.filterStore.setFilter({ title: 'login' });
            const { store, router } = setup(DETAIL);

            router.start('/project/1/issue/view');
            router.redirect(TABLE);

            expect(store.restoreFilter()?.title).toBe('login');
        });

        it('ignores filters changed outside a list view', () => {
            const { store, filterStore, router } = setup(DETAIL);
            filterStore.setInitialFilter(baseFilter());
            filterStore.setFilter({ title: 'from the detail' });

            router.go(TABLE);

            expect(store.restoreFilter()).toBeNull();
        });
    });

    describe('position', () => {
        const position = { loadedCount: 120, scrollTop: 900, scrollLeft: 0 };

        function leaveTable(harness: Harness): void {
            harness.store.registerPosition(() => position);
            harness.router.go(DETAIL);
        }

        it('brings back the loaded rows and scroll on the browser back button', () => {
            const harness = setup(TABLE);
            leaveTable(harness);

            harness.router.go(TABLE, 'popstate');

            expect(harness.store.restorePosition()).toEqual(position);
        });

        it('reads the position when the navigation starts, before the view goes away', () => {
            const harness = setup(TABLE);
            let scrollTop = 900;
            harness.store.registerPosition(() => ({ loadedCount: 120, scrollTop, scrollLeft: 0 }));

            harness.router.start(DETAIL);
            scrollTop = 0;
            harness.router.end(DETAIL);
            harness.router.go(TABLE, 'popstate');

            expect(harness.store.restorePosition()?.scrollTop).toBe(900);
        });

        it('starts from the top when the list is opened through a link', () => {
            const harness = setup(TABLE);
            leaveTable(harness);

            harness.router.go(TABLE);

            expect(harness.store.restorePosition()).toBeNull();
        });

        it('hands the position out once', () => {
            const harness = setup(TABLE);
            leaveTable(harness);
            harness.router.go(TABLE, 'popstate');

            harness.store.restorePosition();

            expect(harness.store.restorePosition()).toBeNull();
        });

        it('does not hand one view mode the position of another', () => {
            const harness = setup(TABLE);
            leaveTable(harness);

            harness.router.go(KANBAN, 'popstate');

            expect(harness.store.restorePosition()).toBeNull();
        });

        it('keeps nothing once the view has unregistered', () => {
            const harness = setup(TABLE);
            const read = () => position;
            harness.store.registerPosition(read);

            harness.store.unregisterPosition(read);
            harness.router.go(DETAIL);
            harness.router.go(TABLE, 'popstate');

            expect(harness.store.restorePosition()).toBeNull();
        });

        it('ignores an unregister from a view that was already replaced', () => {
            const harness = setup(TABLE);
            const previous = () => ({ ...position, loadedCount: 1 });
            harness.store.registerPosition(previous);
            harness.store.registerPosition(() => position);

            harness.store.unregisterPosition(previous);
            harness.router.go(DETAIL);
            harness.router.go(TABLE, 'popstate');

            expect(harness.store.restorePosition()).toEqual(position);
        });
    });
});
