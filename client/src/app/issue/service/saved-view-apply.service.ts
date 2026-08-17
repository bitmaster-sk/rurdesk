import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';
import { SavedView } from '../../project/model/saved-view.model';
import { SavedViewConfigConverter } from '../../project/model/saved-view.converter';
import { SavedViewStore } from '../../project/store/saved-view.store';
import { IssueViewMode } from '../constants/issue-view-modes.enum';
import { IssueFilterStore } from '../components/filter/issue-filter.store';

/**
 * Two apply paths, because a query-param-only navigation reuses the routed child instead of
 * re-instantiating it (query-param-route-reuse.browser.spec.ts): a different view type gets
 * a pending record the mounting component consumes, the same view type gets the filter
 * pushed straight into the store — nothing there would ever consume a pending record.
 */
@Injectable()
export class SavedViewApplyService {
    private readonly store = inject(SavedViewStore);
    private readonly filterStore = inject(IssueFilterStore);
    private readonly router = inject(Router);

    public apply(view: SavedView, idProject: number): void {
        // Before navigating in both branches: the deep-link handler short-circuits on
        // appliedId, so the ?view= param written below cannot trigger a second apply.
        this.store.setApplied(view.idSavedView);

        if (view.viewType !== this.currentMode()) {
            this.store.setPending(view, idProject);
            void this.router.navigate(['/project', idProject, 'issue', 'view', view.viewType], {
                queryParams: { view: view.idSavedView }
            });
            return;
        }

        this.filterStore.setInitialFilter({
            ...SavedViewConfigConverter.toFilter(view.config),
            idProject
        });
        this.markUrl(view.idSavedView);
    }

    /** Zero commands keeps the current route and swaps only the query params. */
    public markUrl(idSavedView: number | null): void {
        void this.router.navigate([], {
            queryParams: { view: idSavedView },
            queryParamsHandling: 'merge'
        });
    }

    /**
     * Read off the URL, not from ActivatedRoute: this service is module-scoped, so its
     * injector would resolve whichever route happened to instantiate it first.
     */
    public currentMode(): IssueViewMode {
        const segment = /\/issue\/view\/([^/?#]+)/.exec(this.router.url)?.[1];
        const modes: string[] = Object.values(IssueViewMode);
        return modes.includes(segment ?? '') ? (segment as IssueViewMode) : IssueViewMode.TABLE;
    }
}
