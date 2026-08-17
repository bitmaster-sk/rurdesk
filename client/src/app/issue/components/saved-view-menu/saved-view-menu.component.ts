import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    ElementRef,
    computed,
    inject,
    input,
    signal,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { UserService } from 'src/app/auth/user.service';
import { SavedViewApi } from 'src/app/project/api/saved-view.api.service';
import { SavedView, SavedViewConfig } from 'src/app/project/model/saved-view.model';
import {
    SAVED_VIEW_DEFAULT_ORDER,
    SavedViewConfigConverter
} from 'src/app/project/model/saved-view.converter';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { AclStore } from 'src/app/project/store/acl.store';
import { UiPopoverComponent } from 'src/app/ui/components/popover/popover.component';
import { UiToastService } from 'src/app/ui/service/ui-toast.service';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { IssueViewMode } from '../../constants/issue-view-modes.enum';
import { SavedViewApplyService } from '../../service/saved-view-apply.service';

/** The names the left menu uses; `kanban` is shown as "Board" everywhere else. */
const VIEW_TYPE_LABELS: Record<IssueViewMode, string> = {
    [IssueViewMode.TABLE]: 'HOME.TABLE',
    [IssueViewMode.KANBAN]: 'HOME.BOARD',
    [IssueViewMode.CALENDAR]: 'HOME.CALENDAR',
    [IssueViewMode.GANTT]: 'HOME.GANTT'
};

export interface SavedViewFormValue {
    name: string;
    isShared: boolean;
}

@Component({
    selector: 'app-saved-view-menu',
    templateUrl: './saved-view-menu.component.html',
    styleUrls: ['./saved-view-menu.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SavedViewMenuComponent {
    private readonly store = inject(SavedViewStore);
    private readonly applyService = inject(SavedViewApplyService);
    private readonly api = inject(SavedViewApi);
    private readonly filterStore = inject(IssueFilterStore);
    private readonly router = inject(Router);
    private readonly toast = inject(UiToastService);
    private readonly i18n = inject(I18nService);
    private readonly userService = inject(UserService);
    private readonly destroyRef = inject(DestroyRef);
    protected readonly aclStore = inject(AclStore);

    public readonly idProject = input.required<number>();

    private readonly panel = viewChild<UiPopoverComponent>('panel');
    private readonly panelEl = viewChild<ElementRef<HTMLElement>>('panelContent');
    private readonly triggerEl = viewChild<ElementRef<HTMLElement>>('trigger');
    private readonly searchEl = viewChild<ElementRef<HTMLInputElement>>('search');

    protected readonly activeView = this.store.appliedView;
    protected readonly hasUnsavedChanges = this.store.isUnsaved;
    protected readonly isOpen = signal(false);
    protected readonly query = signal('');
    /** Row whose delete confirm popup is open — its action strip must stay visible. */
    protected readonly confirmingId = signal<number | null>(null);

    protected readonly isDialogVisible = signal(false);
    protected readonly editTarget = signal<SavedView | null>(null);

    protected readonly panelId = 'saved-view-panel';

    protected readonly matching = computed(() => {
        const query = this.query().trim().toLowerCase();
        const views = this.store.views();
        return query ? views.filter(view => view.name.toLowerCase().includes(query)) : views;
    });

    protected readonly sharedViews = computed(() => this.matching().filter(view => view.isShared));
    protected readonly privateViews = computed(() =>
        this.matching().filter(view => !view.isShared)
    );
    protected readonly hasMatches = computed(() => this.matching().length > 0);
    protected readonly hasViews = computed(() => this.store.views().length > 0);

    protected readonly canManageActive = computed(() => {
        const active = this.activeView();
        return !!active && this.canManage(active);
    });

    public constructor() {
        this.onFilterEdited();
    }

    protected onTriggerClick(event: Event): void {
        this.isOpen.set(!this.isOpen());
        this.panel()?.toggle(event);
        if (this.isOpen()) {
            // ui-popover has no focus machinery, so without this the panel is unreachable
            // by keyboard: Tab from the trigger goes on to the rest of the toolbar.
            setTimeout(() => this.searchEl()?.nativeElement.focus());
        }
    }

    protected onPanelHide(): void {
        // Only reclaim focus if the panel still held it — otherwise a click on another
        // toolbar control lands and focus then jumps back here.
        const heldFocus = this.panelEl()?.nativeElement.contains(document.activeElement);
        this.isOpen.set(false);
        this.query.set('');
        this.confirmingId.set(null);
        if (heldFocus) {
            this.triggerEl()?.nativeElement.focus();
        }
    }

    protected onSearchEnter(event: Event): void {
        event.preventDefault(); // an unhandled Enter would submit an ancestor form
        const matches = this.matching();
        if (matches.length === 1) {
            this.onApply(matches[0]);
        }
    }

    protected onApply(view: SavedView): void {
        this.applyService.apply(view, this.idProject());
        this.closePanel();
    }

    protected onClearActive(): void {
        this.stopFollowing();
        this.store.sendFilterResetSignal();
    }

    protected viewTypeLabel(viewType: IssueViewMode): string {
        return VIEW_TYPE_LABELS[viewType];
    }

    protected canManage(view: SavedView): boolean {
        return view.createBy === this.currentUserId() || this.aclStore.canUpdateProject();
    }

    protected onCopyLink(view: SavedView): void {
        const path = this.router.serializeUrl(
            this.router.createUrlTree(
                ['/project', this.idProject(), 'issue', 'view', view.viewType],
                { queryParams: { view: view.idSavedView } }
            )
        );
        navigator.clipboard
            ?.writeText(`${location.origin}${path}`)
            .then(() => this.notify('ISSUE.VIEWS.LINK_COPIED'))
            .catch(() => this.notify('ISSUE.VIEWS.LINK_COPY_FAILED', 'error'));
    }

    protected onRename(view: SavedView): void {
        this.editTarget.set(view);
        this.openDialog();
    }

    protected onToggleShared(view: SavedView): void {
        this.persist(view, { name: view.name, isShared: !view.isShared }, view.config);
    }

    protected onDelete(view: SavedView): void {
        this.confirmingId.set(null);
        this.api.delete$(view.idSavedView).subscribe(() => {
            if (this.store.appliedView()?.idSavedView === view.idSavedView) {
                // Leaving the id in the URL makes the reloaded list report the view as
                // missing, so "deleted" and "no longer exists" arrive back to back.
                this.store.clearApplied();
                this.applyService.markUrl(null);
            }
            this.store.load(this.idProject());
            this.notify('ISSUE.VIEWS.DELETED_TOAST');
        });
    }

    protected onSaveCurrent(): void {
        this.editTarget.set(null);
        this.openDialog();
    }

    protected onUpdateActive(): void {
        const active = this.activeView();
        if (!active) {
            return;
        }
        this.persist(active, { name: active.name, isShared: active.isShared }, this.liveConfig());
    }

    protected onDialogSaved(value: SavedViewFormValue): void {
        this.isDialogVisible.set(false);
        const target = this.editTarget();
        if (target) {
            // Rename/reshare keeps the stored config, not the filter that happens to be live.
            this.persist(target, value, target.config);
            return;
        }
        this.api
            .create$(this.idProject(), {
                name: value.name,
                viewType: this.applyService.currentMode(),
                config: this.liveConfig(),
                isShared: value.isShared
            })
            .subscribe(created => {
                this.store.setApplied(created.idSavedView);
                this.applyService.markUrl(created.idSavedView);
                this.afterSave();
            });
    }

    protected onDialogCancelled(): void {
        this.isDialogVisible.set(false);
    }

    private onFilterEdited(): void {
        this.filterStore.isFilterEdited$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.store.markUnsaved());
    }

    private openDialog(): void {
        // The dialog's backdrop swallows the outside click that would dismiss the
        // popover, so it would otherwise sit open behind the modal.
        this.closePanel();
        this.isDialogVisible.set(true);
    }

    private stopFollowing(): void {
        if (this.store.idAppliedView() === null) {
            return;
        }
        this.store.clearApplied();
        this.applyService.markUrl(null);
    }

    private persist(view: SavedView, value: SavedViewFormValue, config: SavedViewConfig): void {
        this.api
            .edit$(view.idSavedView, {
                name: value.name,
                viewType: view.viewType,
                config,
                isShared: value.isShared
            })
            .subscribe(() => this.afterSave());
    }

    private liveConfig(): SavedViewConfig {
        const current = this.filterStore.getFilter() ?? {
            ...SAVED_VIEW_DEFAULT_ORDER,
            idProject: this.idProject()
        };
        return SavedViewConfigConverter.toConfig(
            current,
            this.store.liveKanbanLayout() ?? undefined
        );
    }

    private afterSave(): void {
        this.store.markSaved();
        this.store.load(this.idProject());
        this.notify('ISSUE.VIEWS.SAVED_TOAST');
    }

    private notify(key: string, severity: 'success' | 'error' = 'success'): void {
        this.toast.show({ severity, detail: this.i18n.instant(key) });
    }

    private currentUserId(): number | null {
        return this.userService.user.getValue()?.idUser ?? null;
    }

    private closePanel(): void {
        this.panel()?.hide();
    }
}
