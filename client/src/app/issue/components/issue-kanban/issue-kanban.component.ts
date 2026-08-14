import {
    AfterViewInit,
    ChangeDetectionStrategy,
    Component,
    computed,
    DestroyRef,
    inject,
    OnDestroy,
    OnInit,
    signal,
    TemplateRef,
    viewChild
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { SavedViewKanbanLayout } from 'src/app/project/model/saved-view.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import {
    prefersReducedMotion,
    pulseElement,
    UI_SETTLE_DURATION_MS,
    UI_SETTLE_EASING
} from 'src/app/ui/util/motion';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { Notice } from 'src/app/shared/notice/model/notice.model';
import { HttpErrorResponse } from '@angular/common/http';
import { TranslateService } from '@ngx-translate/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { combineLatest } from 'rxjs';
import { first, map } from 'rxjs/operators';
import { IssueService } from '../../issue.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { CdkDragDrop } from '@angular/cdk/drag-drop';
import { ProjectStore } from 'src/app/project/project.store';
import { IssueKanbanService } from './service/issue-kanban.service';
import { KanbanColumn } from './entity/kanban-column.entity';
import { KanbanTile } from './entity/kanban-tile.entity';
import { SwimlaneCell } from './entity/swimlane-cell.entity';
import { SavedViewConfigConverter } from 'src/app/project/model/saved-view.converter';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { Issue } from '../../model/issue.model';
import { SprintStore } from '../../store/sprint.store';
import { SprintAnalyticsStore } from '../../store/sprint-analytics.store';
import { SprintApi } from '../../api/sprint.api.service';
import { SprintTab } from '../sprint-tab-strip/sprint-tab-strip.component';
import { SprintDialogSave } from '../sprint-dialog/sprint-dialog.component';
import { Sprint } from '../../model/sprint.model';
import { SprintUnit } from '../../constants/sprint-unit.enum';
import { SprintState } from '../../constants/sprint-state.enum';
import { IssueQuickActionsComponent } from '../issue-quick-actions/issue-quick-actions.component';

@Component({
    selector: 'app-issue-kanban',
    templateUrl: './issue-kanban.component.html',
    styleUrls: ['./issue-kanban.component.scss'],
    providers: [IssueKanbanService, SprintAnalyticsStore],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class IssueKanbanComponent implements OnInit, AfterViewInit, OnDestroy {
    private readonly projectStore = inject(ProjectStore);

    private readonly issueService = inject(IssueService);

    private readonly issueKanbanService = inject(IssueKanbanService);

    private readonly issueFilterStore = inject(IssueFilterStore);

    private readonly savedViewStore = inject(SavedViewStore);

    private readonly sprintStore = inject(SprintStore);

    private readonly analytics = inject(SprintAnalyticsStore);

    private readonly sprintApi = inject(SprintApi);

    private readonly issueToolbarService = inject(IssueToolbarService);

    private readonly i18n = inject(TranslateService);

    private readonly toast = inject(ToastNotificationService);

    private readonly noticeService = inject(NoticeService);

    private readonly destroyRef = inject(DestroyRef);

    private readonly toolbarRef = viewChild.required<TemplateRef<unknown>>('toolbar');

    private readonly quickActionsRef =
        viewChild.required<IssueQuickActionsComponent>('quickActions');

    protected readonly columns$ = this.issueKanbanService.columns$;

    protected readonly showFilter$ = this.issueFilterStore.showFilter$;

    protected readonly viewMode = signal<SavedViewKanbanLayout>('swimlane');

    protected readonly viewModeOptions = [
        { label: this.i18n.instant('ISSUE.KANBAN.LAYOUT.COLUMNS'), value: 'columns' },
        { label: this.i18n.instant('ISSUE.KANBAN.LAYOUT.SWIMLANE'), value: 'swimlane' }
    ];

    protected readonly swimlaneData$ = combineLatest([
        this.issueKanbanService.swimlaneRows$,
        this.issueKanbanService.states$
    ]).pipe(map(([rows, states]) => ({ rows, states })));

    protected readonly sprints = toSignal(this.sprintStore.sprints$, { initialValue: [] });

    private readonly currentSprint = toSignal(this.sprintStore.currentSprint$, {
        initialValue: undefined
    });

    protected readonly selectedIdSprint = signal<number | null>(null);

    protected readonly selectedSprint = computed<Sprint | null>(
        () => this.sprints().find(s => s.idSprint === this.selectedIdSprint()) ?? null
    );

    protected readonly unit = signal(SprintUnit.Points);

    protected readonly stats = this.analytics.stats;

    protected readonly velocities = this.analytics.velocities;

    protected readonly burndown = this.analytics.burndown;

    protected readonly isBurndownLoading = this.analytics.isBurndownLoading;

    protected readonly projectName = signal('');

    private static readonly showChartsKey = 'issue-kanban-show-charts';

    protected readonly showCharts = signal(
        localStorage.getItem(IssueKanbanComponent.showChartsKey) === 'true'
    );

    // Display setting: show closed sprints as (read-only) tabs. Pure display
    // preference, persisted per user in localStorage like the table settings.
    private static readonly showClosedSprintsKey = 'issue-kanban-show-closed-sprints';

    protected readonly showClosedSprints = signal(
        localStorage.getItem(IssueKanbanComponent.showClosedSprintsKey) === 'true'
    );

    protected readonly sprintTabs = computed<SprintTab[]>(() => {
        const list = this.sprints();
        const current = this.currentSprint();
        const byStart = (
            a: { startAt: string; idSprint: number },
            b: { startAt: string; idSprint: number }
        ): number => a.startAt.localeCompare(b.startAt) || a.idSprint - b.idSprint;
        const ordered = list
            .filter(s => s.state !== SprintState.Closed)
            .slice()
            .sort(byStart);
        const closed = this.showClosedSprints()
            ? list
                  .filter(s => s.state === SprintState.Closed)
                  .slice()
                  .sort((a, b) => b.endAt.localeCompare(a.endAt) || b.idSprint - a.idSprint)
            : [];
        return [
            {
                idSprint: null,
                label: this.i18n.instant('ISSUE.KANBAN.SPRINTS.BACKLOG'),
                isCurrent: false,
                isClosed: false,
                listId: 'sprint-tab-backlog'
            },
            ...[...ordered, ...closed].map(s => ({
                idSprint: s.idSprint,
                label: s.name,
                isCurrent: current?.idSprint === s.idSprint,
                isClosed: s.state === SprintState.Closed,
                listId: `sprint-tab-${s.idSprint}`
            }))
        ];
    });

    // Closed sprints are immutable history — never drop targets.
    protected readonly sprintTabListIds = computed(() =>
        this.sprintTabs()
            .filter(t => !t.isClosed)
            .map(t => t.listId)
    );

    protected readonly isSelectedSprintClosed = computed(
        () => this.selectedSprint()?.state === SprintState.Closed
    );

    protected readonly dialogOpen = signal(false);
    protected readonly editingSprint = signal<Sprint | null>(null);

    private readonly idProject = signal(0);

    protected readonly isSplitDialogOpen = signal(false);

    protected readonly splitIssue = signal<Issue | null>(null);

    protected readonly sortColumn = signal('title');

    protected readonly sortDirection = signal<'asc' | 'desc'>('asc');

    protected readonly sortOptions = [
        { label: this.i18n.instant('TITLE'), value: 'title' },
        { label: this.i18n.instant('STATE.SINGULAR'), value: 'state' },
        { label: this.i18n.instant('SEVERITY.SINGULAR'), value: 'severity' },
        { label: this.i18n.instant('UPDATED.AT'), value: 'updateAt' },
        { label: this.i18n.instant('CREATED.AT'), value: 'createAt' }
    ];

    protected readonly currentSortLabel = computed(
        () => this.sortOptions.find(o => o.value === this.sortColumn())?.label ?? ''
    );

    protected readonly sortMenuItems = computed(() =>
        this.sortOptions.map(o => ({
            label: o.label,
            command: () => this.onSortColumnChange(o.value)
        }))
    );

    public ngOnInit(): void {
        this.savedViewStore.setLiveKanbanLayout(this.viewMode());
        this.setInitialFilter();
        this.onSavedViewResetSignal();
        this.applyViewLayoutOnChange();

        // Live updates: apply teammate changes to the loaded board. A move
        // between columns FLIP-flies the card; own drags land as in-place
        // updates (the optimistic move already happened) and don't re-animate.
        this.noticeService.issue$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(notice => this.onRemoteIssueNotice(notice));
    }

    private onRemoteIssueNotice(notice: Notice<Issue>): void {
        const issue = notice.payload;
        if (!issue || issue.idProject !== this.idProject()) return;
        this.analytics.reloadStatsAfterNotice();
        if (this.showCharts() && this.selectedIdSprint() !== null) {
            this.analytics.reloadBurndownAfterNotice();
        }

        if (notice.action !== NoticeAction.Update) {
            this.issueFilterStore.refresh();
            return;
        }

        const sourceRect = (
            document.querySelector(`[data-tile-id="${issue.idIssue}"]`) as HTMLElement | null
        )?.getBoundingClientRect();

        const result = this.issueKanbanService.applyRemoteIssue(
            issue,
            this.selectedIdSprint(),
            this.viewMode()
        );
        if (result === 'missing') {
            this.issueFilterStore.refresh();
            return;
        }
        if (result === 'moved' && sourceRect) {
            this.flyTile(issue.idIssue, sourceRect);
            return;
        }
        if (result === 'updated') {
            // Own drag echoes land here too — the pulse doubles as the "change
            // persisted" confirmation, so every update pulses, drops included.
            this.pulseTile(issue.idIssue);
        }
    }

    /** Shared "this just changed" ring on an in-place updated tile. */
    private pulseTile(idIssue: number): void {
        requestAnimationFrame(() => {
            const el = document.querySelector(`[data-tile-id="${idIssue}"]`) as HTMLElement | null;
            if (el) pulseElement(el);
        });
    }

    /** Cross-column FLIP: the re-rendered tile glides from its old rect. */
    private flyTile(idIssue: number, from: DOMRect): void {
        if (prefersReducedMotion()) return;
        // In a hidden tab rAF stalls until the tab is shown again — the flight
        // would replay on return, long after the change. Just land the tile.
        if (document.hidden) return;
        // One retry: change detection may not have re-rendered the moved tile
        // by the first frame; if it still sits at the old rect, check next frame.
        const attempt = (retriesLeft: number): void => {
            requestAnimationFrame(() => {
                const el = document.querySelector(
                    `[data-tile-id="${idIssue}"]`
                ) as HTMLElement | null;
                if (!el) return;
                const to = el.getBoundingClientRect();
                const dx = from.left - to.left;
                const dy = from.top - to.top;
                if (dx === 0 && dy === 0) {
                    if (retriesLeft > 0) attempt(retriesLeft - 1);
                    return;
                }
                el.classList.add('tile-flying');
                const animation = el.animate(
                    [
                        {
                            transform: `translate(${dx}px, ${dy}px) scale(1.03)`,
                            boxShadow: '0 8px 24px rgba(0, 0, 0, 0.18)'
                        },
                        {
                            transform: 'translate(0, 0) scale(1)',
                            boxShadow: '0 0 0 rgba(0, 0, 0, 0)'
                        }
                    ],
                    { duration: UI_SETTLE_DURATION_MS, easing: UI_SETTLE_EASING }
                );
                animation.finished
                    .catch(() => undefined)
                    .then(() => el.classList.remove('tile-flying'));
            });
        };
        attempt(1);
    }

    private shiftTotals(evt: CdkDragDrop<KanbanColumn> | CdkDragDrop<SwimlaneCell>): void {
        if (evt.previousContainer === evt.container) return;
        evt.previousContainer.data.total = Math.max(0, evt.previousContainer.data.total - 1);
        evt.container.data.total += 1;
    }

    public ngAfterViewInit(): void {
        this.issueToolbarService.register(this.toolbarRef());
    }

    public ngOnDestroy(): void {
        this.issueToolbarService.clear();
        // left behind, it would land in a view saved from the table or the calendar
        this.savedViewStore.setLiveKanbanLayout(null);
    }

    protected onToggleFilter(): void {
        this.issueFilterStore.toggleShowFilter();
    }

    protected onSortColumnChange(column: string): void {
        this.sortColumn.set(column);
        this.issueFilterStore.setOrder({
            orderColumn: column,
            orderDirection: this.sortDirection()
        });
    }

    protected onSortDirectionToggle(): void {
        const dir = this.sortDirection() === 'asc' ? 'desc' : 'asc';
        this.sortDirection.set(dir);
        this.issueFilterStore.setOrder({ orderColumn: this.sortColumn(), orderDirection: dir });
    }

    protected onContextMenuRequested({ event, issue }: { event: MouseEvent; issue: Issue }): void {
        this.quickActionsRef().show(event, issue);
    }

    protected onSplitRequested(issue: Issue): void {
        this.splitIssue.set(issue);
        this.isSplitDialogOpen.set(true);
    }

    protected onSplitAccepted(): void {
        this.isSplitDialogOpen.set(false);
    }

    protected onSplitCancelled(): void {
        this.isSplitDialogOpen.set(false);
    }

    protected onColumnLoadMore(column: KanbanColumn): void {
        this.issueKanbanService.loadMoreColumn(column);
    }

    protected onCellLoadMore(cell: SwimlaneCell): void {
        this.issueKanbanService.loadMoreCell(cell);
    }

    protected onSprintChange(idSprint: number | null): void {
        this.selectedIdSprint.set(idSprint);
        this.issueFilterStore.setSprint(idSprint);
        this.analytics.scopeAndReload(this.idProject(), idSprint);
        if (this.showCharts()) {
            this.analytics.reloadBurndown();
        }
    }

    protected onToggleCharts(value: boolean): void {
        this.showCharts.set(value);
        localStorage.setItem(IssueKanbanComponent.showChartsKey, String(value));
        if (value) {
            this.analytics.reloadBurndown();
        }
    }

    protected onShowClosedSprintsChange(value: boolean): void {
        this.showClosedSprints.set(value);
        localStorage.setItem(IssueKanbanComponent.showClosedSprintsKey, String(value));
        // Never leave the board scoped to a tab that just became invisible.
        if (!value && this.isSelectedSprintClosed()) {
            this.onSprintChange(this.currentSprint()?.idSprint ?? null);
        }
    }

    // Roll over = move the scoped sprint's unfinished tasks to the next planned
    // cycle (bumping their carry-over count) and close it. Reuses the close endpoint.
    protected onRollOver(): void {
        const idSprint = this.selectedIdSprint();
        if (idSprint == null) {
            return;
        }
        this.sprintApi.close$(idSprint).subscribe({
            next: () => {
                this.onSprintChange(null); // closed sprint leaves the strip; reload board unscoped
                this.sprintStore.load(this.idProject());
                this.analytics.reloadVelocity();
                this.toast.showSuccess('ISSUE.KANBAN.SPRINTS.CLOSED_TOAST');
            },
            error: (error: HttpErrorResponse) => {
                this.toast.showError(
                    error.status === 409
                        ? 'ISSUE.KANBAN.SPRINTS.CLOSE_CONFLICT'
                        : 'ISSUE.KANBAN.SPRINTS.CLOSE_FAILED'
                );
            }
        });
    }

    protected onCreateSprint(): void {
        this.editingSprint.set(null);
        this.dialogOpen.set(true);
    }

    protected onEditSprint(idSprint: number): void {
        this.editingSprint.set(this.sprints().find(s => s.idSprint === idSprint) ?? null);
        this.dialogOpen.set(true);
    }

    protected onSprintSaved(payload: SprintDialogSave): void {
        const editing = this.editingSprint();
        if (editing) {
            this.sprintStore.edit(this.idProject(), editing.idSprint, payload);
        } else {
            this.sprintStore.create(this.idProject(), payload);
        }
    }

    protected onSprintDeleted(): void {
        const editing = this.editingSprint();
        if (!editing) {
            return;
        }
        this.sprintStore.remove$(this.idProject(), editing.idSprint).subscribe(() => {
            if (this.selectedIdSprint() === editing.idSprint) {
                this.onSprintChange(null);
            }
        });
    }

    // A task dragged from any board list onto a sprint tab → assign it to that
    // sprint (or clear to Backlog when idSprint is null). id_sprint has its own
    // endpoint (issue update never carries it), then reload to reconcile.
    protected onTabTaskDropped(payload: {
        idSprint: number | null;
        event: CdkDragDrop<SprintTab>;
    }): void {
        // The tab is a cdkDropList, so by drop time CDK has already reparented the
        // dragged tile's DOM node into the tab button. Every path must re-render the
        // source list so Angular reclaims that node — otherwise the orphaned tile
        // renders at the top-left corner until the next reload.
        const source = payload.event.previousContainer.data as {
            tiles?: KanbanTile[];
            total?: number;
        };
        const tile = source?.tiles?.[payload.event.previousIndex];
        if (!tile || tile.idIssuePublic == null || (tile.idSprint ?? null) === payload.idSprint) {
            this.issueFilterStore.refresh();
            return;
        }

        // Assigning to another sprint moves the task out of the current (single-sprint)
        // board scope — drop it from the source list optimistically, mirroring the
        // state-change drops, then reconcile with the server.
        source.tiles!.splice(payload.event.previousIndex, 1);
        if (source.total != null) source.total = Math.max(0, source.total - 1);

        this.sprintApi
            .assignIssue$(tile.idProject, tile.idIssuePublic, payload.idSprint)
            .subscribe({
                next: () => {
                    this.issueFilterStore.refresh();
                    this.analytics.reloadStats();
                    if (this.showCharts()) {
                        this.analytics.reloadBurndown();
                    }
                },
                error: () => {
                    this.issueFilterStore.refresh();
                }
            });
    }

    protected onStateChange(evt: CdkDragDrop<KanbanColumn>): void {
        const issue = evt.previousContainer.data.tiles[evt.previousIndex];
        const newState = evt.container.data.state;
        issue.state = newState;
        issue.idState = newState.idState;
        evt.previousContainer.data.tiles.splice(evt.previousIndex, 1);
        evt.container.data.tiles.unshift(issue);
        this.shiftTotals(evt);
        this.refreshBurndown();
        this.issueService.updateIssue(issue).subscribe({
            error: () => {
                this.issueFilterStore.refresh();
            }
        });
    }

    protected onSwimlaneCardDrop(evt: CdkDragDrop<SwimlaneCell>): void {
        const tile = evt.previousContainer.data.tiles[evt.previousIndex];
        const { state: newState, user: newUser } = evt.container.data;

        const stateChanged = tile.idState !== newState.idState;
        const userChanged = tile.assignedTo !== newUser?.idUser;

        if (!stateChanged && !userChanged) return;

        const updated: KanbanTile = {
            ...tile,
            idState: newState.idState,
            state: newState,
            assignedTo: newUser?.idUser,
            assignedToUser: newUser
        };

        evt.previousContainer.data.tiles.splice(evt.previousIndex, 1);
        evt.container.data.tiles.unshift(updated);
        this.shiftTotals(evt);
        this.refreshBurndown();
        this.issueService.updateIssue(updated).subscribe({
            error: () => {
                this.issueFilterStore.refresh();
            }
        });
    }

    private refreshBurndown(): void {
        if (this.showCharts() && this.selectedIdSprint() !== null) {
            this.analytics.reloadBurndown();
        }
    }

    protected onViewModeChange(layout: SavedViewKanbanLayout): void {
        this.viewMode.set(layout);
        this.savedViewStore.setLiveKanbanLayout(layout);
    }

    private onSavedViewResetSignal(): void {
        this.savedViewStore.filterResetSignal$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.setDefaultFilter(this.idProject(), this.selectedIdSprint()));
    }

    /** Both apply paths end in setInitialFilter, so this covers a remount and a same-mode push. */
    private applyViewLayoutOnChange(): void {
        this.issueFilterStore.initialFilter$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => {
                const layout = this.savedViewStore.appliedView()?.config.kanbanLayout;
                if (layout) {
                    this.onViewModeChange(layout);
                }
            });
    }

    private setInitialFilter(): void {
        this.projectStore.project$.pipe(first()).subscribe(project => {
            this.idProject.set(project.idProject);
            this.projectName.set(project.name);
            this.sprintStore.currentSprintOnLoad$
                .pipe(first(), takeUntilDestroyed(this.destroyRef))
                .subscribe(current => this.onSprintChange(current?.idSprint ?? null));
            this.sprintStore.load(project.idProject);
            this.analytics.setScope(project.idProject, this.selectedIdSprint());
            this.analytics.reloadVelocity();

            const pending = this.savedViewStore.consumePending(project.idProject);
            if (pending) {
                // A staged view REPLACES the defaults: a field it omits is unfiltered.
                this.issueFilterStore.setInitialFilter({
                    ...SavedViewConfigConverter.toFilter(pending.config),
                    idProject: project.idProject
                });
            } else {
                this.setDefaultFilter(project.idProject);
            }
        });
    }

    private setDefaultFilter(idProject: number, idSprint?: number | null): void {
        this.issueFilterStore.setInitialFilter({
            idProject,
            ...(idSprint === undefined ? {} : { idSprint, sprintUnset: idSprint === null }),
            orderColumn: 'title',
            orderDirection: 'asc',
            idsState: [],
            idsSeverity: [],
            idsAssignedTo: [],
            stateUnset: true,
            severityUnset: true,
            assignedToUnset: true,
            createAtFrom: null,
            createAtTo: null,
            updateAtFrom: null,
            updateAtTo: null
        });
    }
}
