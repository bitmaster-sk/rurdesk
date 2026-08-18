import { Injector } from '@angular/core';
import { Router } from '@angular/router';
import { of } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SavedViewApi } from '../../project/api/saved-view.api.service';
import { SavedView } from '../../project/model/saved-view.model';
import { SavedViewStore } from '../../project/store/saved-view.store';
import { IssueViewMode } from '../constants/issue-view-modes.enum';
import { IssueFilterStore } from '../components/filter/issue-filter.store';
import { SavedViewApplyService } from './saved-view-apply.service';

function savedView(overrides: Partial<SavedView> = {}): SavedView {
    return {
        idSavedView: 7,
        idProject: 1,
        name: 'My bugs',
        viewType: IssueViewMode.TABLE,
        isShared: false,
        createBy: 1,
        updateAt: '2026-08-01T00:00:00Z',
        config: { v: 1, idsState: [1, 2] },
        ...overrides
    };
}

describe('SavedViewApplyService', () => {
    let navigate: ReturnType<typeof vi.fn>;
    let setInitialFilter: ReturnType<typeof vi.fn>;
    let service: SavedViewApplyService;
    let store: SavedViewStore;
    let calls: string[];
    let url: string;

    function build(currentUrl: string): void {
        url = currentUrl;
        calls = [];
        navigate = vi.fn(() => {
            calls.push('navigate');
            return Promise.resolve(true);
        });
        setInitialFilter = vi.fn(() => calls.push('setInitialFilter'));

        const injector = Injector.create({
            providers: [
                { provide: SavedViewApi, useValue: { loadByProject$: vi.fn(() => of([])) } },
                { provide: SavedViewStore, useClass: SavedViewStore },
                { provide: IssueFilterStore, useValue: { setInitialFilter } },
                {
                    provide: Router,
                    useValue: {
                        navigate,
                        get url(): string {
                            return url;
                        }
                    }
                },
                { provide: SavedViewApplyService, useClass: SavedViewApplyService }
            ]
        });
        service = injector.get(SavedViewApplyService);
        store = injector.get(SavedViewStore);
        const setApplied = store.setApplied.bind(store);
        store.setApplied = (value: number) => {
            calls.push('setApplied');
            setApplied(value);
        };
    }

    beforeEach(() => build('/project/1/issue/view/table'));

    it('reads the current view type out of the URL', () => {
        expect(service.currentMode()).toBe(IssueViewMode.TABLE);

        build('/project/1/issue/view/kanban?view=3');
        expect(service.currentMode()).toBe(IssueViewMode.KANBAN);
    });

    it('falls back to the table for a URL with no view segment', () => {
        build('/project/1/view');
        expect(service.currentMode()).toBe(IssueViewMode.TABLE);
    });

    it('same view type pushes the filter and swaps only the query param', () => {
        service.apply(savedView(), 1);

        expect(setInitialFilter).toHaveBeenCalledWith({
            idProject: 1,
            idsState: [1, 2],
            orderColumn: 'updateAt',
            orderDirection: 'desc'
        });
        expect(navigate).toHaveBeenCalledWith([], {
            queryParams: { view: 7 },
            queryParamsHandling: 'merge'
        });
        expect(store.consumePending(1)).toBeNull(); // nothing would ever consume it
    });

    it('different view type stages a pending record and navigates to that view', () => {
        const view = savedView({ viewType: IssueViewMode.KANBAN });

        service.apply(view, 1);

        expect(setInitialFilter).not.toHaveBeenCalled();
        expect(navigate).toHaveBeenCalledWith(['/project', 1, 'issue', 'view', 'kanban'], {
            queryParams: { view: 7 }
        });
        expect(store.consumePending(1)).toBe(view);
    });

    // The deep-link handler reads appliedId, so navigation must not beat it there.
    it.each([IssueViewMode.TABLE, IssueViewMode.KANBAN])(
        'sets appliedId BEFORE navigating (view type %s)',
        viewType => {
            service.apply(savedView({ viewType }), 1);

            expect(store.idAppliedView()).toBe(7);
            expect(calls.indexOf('setApplied')).toBeLessThan(calls.indexOf('navigate'));
        }
    );

    it('revives absolute dates and keeps rolling windows as strings', () => {
        service.apply(
            savedView({
                config: { v: 1, createAtFrom: '2026-01-01T00:00:00.000Z', updateAtWithin: '30d' }
            }),
            1
        );

        const applied = setInitialFilter.mock.calls[0][0];
        expect(applied.createAtFrom).toBeInstanceOf(Date);
        expect(applied.updateAtWithin).toBe('30d');
    });

    it('defaults the sort for a config that has none', () => {
        service.apply(savedView({ config: { v: 1 } }), 1);

        const applied = setInitialFilter.mock.calls[0][0];
        expect(applied.orderColumn).toBe('updateAt');
        expect(applied.orderDirection).toBe('desc');
    });

    it('applies exactly one filter per apply', () => {
        service.apply(savedView(), 1);
        expect(setInitialFilter).toHaveBeenCalledTimes(1);
    });

    it('markUrl(null) drops the param without touching the route', () => {
        service.markUrl(null);

        expect(navigate).toHaveBeenCalledWith([], {
            queryParams: { view: null },
            queryParamsHandling: 'merge'
        });
    });
});
