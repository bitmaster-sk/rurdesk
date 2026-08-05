import { Injectable, Signal, computed, inject, signal } from '@angular/core';
import { Observable, Subject, of } from 'rxjs';
import { catchError, tap } from 'rxjs/operators';
import { SavedViewApi } from '../api/saved-view.api.service';
import { SavedView, SavedViewKanbanLayout } from '../model/saved-view.model';

/**
 * Root-scoped on purpose: the ⌘K provider registers from ProjectModule and must reach this
 * store before the lazy IssueModule has ever loaded.
 */
@Injectable({ providedIn: 'root' })
export class SavedViewStore {
    private readonly api = inject(SavedViewApi);

    /** The list of views for the current project. */
    private readonly _views = signal<SavedView[]>([]);
    public readonly views: Signal<SavedView[]> = this._views.asReadonly();

    /** The id of the saved view currently applied */
    private readonly _idAppliedView = signal<number | null>(null);
    public readonly idAppliedView: Signal<number | null> = this._idAppliedView.asReadonly();
    /** The saved view currently applied, or null if none. */
    public readonly appliedView = computed(
        () => this._views().find(view => view.idSavedView === this._idAppliedView()) ?? null
    );

    /** Holds whether the current view has unsaved changes. */
    private readonly _isUnsaved = signal(false);
    public readonly isUnsaved: Signal<boolean> = this._isUnsaved.asReadonly();

    /** The project for which the store has loaded views. */
    private readonly _idLoadedProject = signal<number | null>(null);
    public readonly idLoadedProject: Signal<number | null> = this._idLoadedProject.asReadonly();

    /** Kanban layout currently shown on the board. (columns/swimlane) */
    private readonly _liveKanbanLayout = signal<SavedViewKanbanLayout | null>(null);
    public readonly liveKanbanLayout: Signal<SavedViewKanbanLayout | null> =
        this._liveKanbanLayout.asReadonly();

    /** A signal that emits when the filter should be reset. */
    private readonly _filterResetSignal = new Subject<void>();
    public readonly filterResetSignal$: Observable<void> = this._filterResetSignal.asObservable();

    private pending: { view: SavedView; idProject: number } | null = null;

    private requestToken = 0;

    public load(idProject: number): void {
        this.load$(idProject).subscribe();
    }

    public load$(idProject: number): Observable<SavedView[]> {
        this.resetOnProjectChange(idProject);
        const token = ++this.requestToken;
        return this.api.loadByProject$(idProject).pipe(
            tap(views => {
                if (token !== this.requestToken) {
                    return;
                }
                this._views.set(views);
                this._idLoadedProject.set(idProject);
            }),
            catchError(() => of<SavedView[]>([]))
        );
    }

    public setApplied(idSavedView: number): void {
        this._idAppliedView.set(idSavedView);
        this._isUnsaved.set(false);
    }

    public clearApplied(): void {
        this._idAppliedView.set(null);
        this._isUnsaved.set(false);
    }

    public markUnsaved(): void {
        if (this._idAppliedView() !== null) {
            this._isUnsaved.set(true);
        }
    }

    public markSaved(): void {
        this._isUnsaved.set(false);
    }

    public setLiveKanbanLayout(layout: SavedViewKanbanLayout | null): void {
        this._liveKanbanLayout.set(layout);
    }

    public sendFilterResetSignal(): void {
        this._filterResetSignal.next();
    }

    /** Stamped with its project so a cancelled navigation cannot inject foreign ids. */
    public setPending(view: SavedView, idProject: number): void {
        this.pending = { view, idProject };
    }

    /** One-shot: a plain navigation after an apply must not re-apply a stale view. */
    public consumePending(idProject: number): SavedView | null {
        const staged = this.pending;
        this.pending = null;
        if (!staged || staged.idProject !== idProject) {
            return null;
        }
        return staged.view;
    }

    private resetOnProjectChange(idProject: number): void {
        if (this._idLoadedProject() !== null && this._idLoadedProject() !== idProject) {
            this._views.set([]);
            this._idAppliedView.set(null);
            this._isUnsaved.set(false);
            this._liveKanbanLayout.set(null);
            this.pending = null;
        }
        this._idLoadedProject.set(null);
    }
}
