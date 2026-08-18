import { Injector } from '@angular/core';
import { of, Subject, throwError } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { SavedViewApi } from '../api/saved-view.api.service';
import { SavedView } from '../model/saved-view.model';
import { SavedViewStore } from './saved-view.store';

describe('SavedViewStore', () => {
    function makeStore(views: SavedView[] = []): {
        store: SavedViewStore;
        loadByProject$: ReturnType<typeof vi.fn>;
    } {
        const loadByProject$ = vi.fn(() => of(views));
        const injector = Injector.create({
            providers: [
                { provide: SavedViewApi, useValue: { loadByProject$ } },
                { provide: SavedViewStore, useClass: SavedViewStore }
            ]
        });
        return { store: injector.get(SavedViewStore), loadByProject$ };
    }

    function view(idSavedView: number, idProject = 1): SavedView {
        return {
            idSavedView,
            idProject,
            name: `v${idSavedView}`,
            viewType: IssueViewMode.TABLE,
            config: { v: 1 },
            isShared: false,
            createBy: 1,
            updateAt: '2026-08-01T00:00:00Z'
        };
    }

    it('drops a pending record staged for another project', () => {
        const { store } = makeStore();
        store.setPending(view(1), 1);

        expect(store.consumePending(2)).toBeNull();
        expect(store.consumePending(1)).toBeNull();
    });

    it('is one-shot for the matching project', () => {
        const { store } = makeStore();
        const staged = view(1);
        store.setPending(staged, 1);

        expect(store.consumePending(1)).toBe(staged);
        expect(store.consumePending(1)).toBeNull();
    });

    it('derives activeView from appliedId', () => {
        const { store } = makeStore([view(1), view(2)]);
        store.load(1);
        expect(store.appliedView()).toBeNull();

        store.setApplied(2);
        expect(store.appliedView()?.idSavedView).toBe(2);

        store.clearApplied();
        expect(store.appliedView()).toBeNull();
    });

    // Clearing appliedId on a filter edit would make "Update view" unreachable.
    it('flags unsaved changes only while a view is applied', () => {
        const { store } = makeStore([view(1)]);
        store.load(1);

        store.markUnsaved();
        expect(store.isUnsaved()).toBe(false);

        store.setApplied(1);
        store.markUnsaved();
        expect(store.isUnsaved()).toBe(true);
    });

    it.each([
        ['re-applying', (store: SavedViewStore) => store.setApplied(1)],
        ['clearing', (store: SavedViewStore) => store.clearApplied()],
        ['saving', (store: SavedViewStore) => store.markSaved()]
    ])('%s resets the unsaved flag', (_label, act) => {
        const { store } = makeStore([view(1)]);
        store.load(1);
        store.setApplied(1);
        store.markUnsaved();

        act(store);

        expect(store.isUnsaved()).toBe(false);
    });

    it('an appliedId with no matching view resolves to null rather than throwing', () => {
        const { store } = makeStore([view(1)]);
        store.load(1);
        store.setApplied(404);
        expect(store.appliedView()).toBeNull();
    });

    it('resets views, appliedId and pending when the project changes', () => {
        const { store, loadByProject$ } = makeStore([view(1)]);
        store.load(1);
        store.setApplied(1);
        store.setPending(view(1), 1);

        loadByProject$.mockReturnValue(of([]));
        store.load(2);

        expect(store.views()).toEqual([]);
        expect(store.idAppliedView()).toBeNull();
        expect(store.consumePending(2)).toBeNull();
    });

    it('keeps state when loading the same project again', () => {
        const { store } = makeStore([view(1)]);
        store.load(1);
        store.setApplied(1);

        store.load(1);

        expect(store.idAppliedView()).toBe(1);
        expect(store.idLoadedProject()).toBe(1);
    });

    // The ⌘K provider short-circuits on loadedProject, so claiming a project is loaded
    // before its listing arrives leaves the palette permanently on an empty list.
    it('reports a project as loaded only once its listing arrives', () => {
        const pending = new Subject<SavedView[]>();
        const injector = Injector.create({
            providers: [
                { provide: SavedViewApi, useValue: { loadByProject$: () => pending } },
                { provide: SavedViewStore, useClass: SavedViewStore }
            ]
        });
        const store = injector.get(SavedViewStore);

        store.load(1);
        expect(store.idLoadedProject()).toBeNull();

        pending.next([view(1)]);
        expect(store.idLoadedProject()).toBe(1);
    });

    // Two quick project switches can land out of order; A's late response must not
    // overwrite B's views under B's id.
    it('discards a response for a project the user has already left', () => {
        const first = new Subject<SavedView[]>();
        const second = new Subject<SavedView[]>();
        const responses = [first, second];
        const injector = Injector.create({
            providers: [
                { provide: SavedViewApi, useValue: { loadByProject$: () => responses.shift()! } },
                { provide: SavedViewStore, useClass: SavedViewStore }
            ]
        });
        const store = injector.get(SavedViewStore);

        store.load(1);
        store.load(2);
        second.next([view(9, 2)]);
        first.next([view(1, 1)]); // project 1's response arrives late

        expect(store.idLoadedProject()).toBe(2);
        expect(store.views().map(item => item.idSavedView)).toEqual([9]);
    });

    it('a failed load leaves the project unloaded so the next caller retries', () => {
        const injector = Injector.create({
            providers: [
                {
                    provide: SavedViewApi,
                    useValue: { loadByProject$: () => throwError(() => new Error('boom')) }
                },
                { provide: SavedViewStore, useClass: SavedViewStore }
            ]
        });
        const store = injector.get(SavedViewStore);

        store.load(1);

        expect(store.idLoadedProject()).toBeNull();
        expect(store.views()).toEqual([]);
    });
});
