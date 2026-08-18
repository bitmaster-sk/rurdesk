import {
    AfterViewInit,
    ChangeDetectionStrategy,
    Component,
    HostListener,
    OnDestroy,
    TemplateRef,
    effect,
    inject,
    signal,
    viewChild,
    computed,
    DestroyRef
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { Subscription } from 'rxjs';
import { first, filter } from 'rxjs/operators';
import { startOfMonth, subMonths, endOfMonth, addMonths } from 'date-fns';
import { ProjectStore } from 'src/app/project/project.store';
import { SavedViewConfigConverter } from 'src/app/project/model/saved-view.converter';
import { SavedViewStore } from 'src/app/project/store/saved-view.store';
import { IssueFilterStore } from '../filter/issue-filter.store';
import { IssueGanttService } from './service/issue-gantt.service';
import { GanttTimelineService } from './service/gantt-timeline.service';
import { GanttDragService, DragMode } from './service/gantt-drag.service';
import { GanttCascadeService, CascadeResult } from './service/gantt-cascade.service';
import { GanttCriticalPathService, emptyCriticalPath } from './service/gantt-critical-path.service';
import { GanttZoomLevel } from './constants/gantt-zoom-config';
import { HandleSide } from './constants/gantt-handle-side.enum';
import { IssueToolbarService } from '../../issue-toolbar.service';
import { IssueQuickActionsComponent } from '../issue-quick-actions/issue-quick-actions.component';
import { GanttWbsPanelComponent } from './components/gantt-wbs-panel/gantt-wbs-panel';
import { GanttTimelineBodyComponent } from './components/gantt-timeline-body/gantt-timeline-body';
import { Issue } from '../../model/issue.model';
import { IssueBulkApi } from '../../api/issue-bulk.api.service';
import { BulkEditIssueEntry } from '../../model/bulk-edit-issues.model';
import { IssueService } from '../../issue.service';
import { IssueRelationType } from '../../constants/issue-relation-type.enum';
import { IssueRelationSubType } from '../../constants/issue-relation-subtype.enum';
import { IssueRelationDirection } from '../../constants/issue-relation-direction.enum';
import { GanttRelation } from './model/gantt-relation.model';
import { IssueRelationApi } from '../../api/issue-relation.api.service';
import {
    BAR_BORDER,
    BAR_PADDING_COMFORTABLE,
    BAR_PADDING_COMPACT
} from './components/gantt-task-bar/gantt-task-bar';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { NoticeAction } from 'src/app/shared/notice/constant/notice-action.enum';
import { GanttOrderApi } from '../../api/gantt-order.api.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { applyPendingOrder } from './service/gantt-order.util';
import { CommandPaletteService } from 'src/app/core/command/command-palette.service';
import {
    IssueCardViewType,
    GANTT_CARD_MODE_OPTIONS,
    isComfortableMode
} from '../../constants/issue-card-view-type.constant';
import { ZOOM_LEVELS, ZOOM_OPTIONS } from './constants/gantt-zoom-options';
import { STORAGE_KEY_CARD_MODE, STORAGE_KEY_MINIMAP } from './constants/gantt-storage-keys';
import { pulseElement } from 'src/app/ui/util/motion';

const ROW_HEIGHT_COMFORTABLE = 72;
const ROW_HEIGHT_COMPACT = 38;

@Component({
    selector: 'app-issue-gantt',
    templateUrl: './issue-gantt.component.html',
    styleUrls: ['./issue-gantt.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    providers: [
        IssueGanttService,
        GanttTimelineService,
        GanttDragService,
        GanttCascadeService,
        GanttCriticalPathService
    ],
    standalone: false
})
export class IssueGanttComponent implements AfterViewInit, OnDestroy {
    private readonly toolbarRef = viewChild.required<TemplateRef<unknown>>('toolbar');
    private readonly quickActionsRef = viewChild<IssueQuickActionsComponent>('quickActions');
    private readonly wbsPanelRef = viewChild<GanttWbsPanelComponent>('wbsPanel');
    public readonly timelineBodyRef = viewChild<GanttTimelineBodyComponent>('timelineBody');

    private readonly projectStore = inject(ProjectStore);
    private readonly issueFilterStore = inject(IssueFilterStore);
    private readonly savedViewStore = inject(SavedViewStore);
    private readonly ganttService = inject(IssueGanttService);
    public readonly timelineService = inject(GanttTimelineService);
    public readonly dragService = inject(GanttDragService);
    private readonly cascadeService = inject(GanttCascadeService);
    private readonly criticalPathService = inject(GanttCriticalPathService);
    private readonly bulkApi = inject(IssueBulkApi);
    private readonly issueService = inject(IssueService);
    private readonly relationApi = inject(IssueRelationApi);
    private readonly noticeService = inject(NoticeService);
    private readonly issueToolbarService = inject(IssueToolbarService);
    private readonly router = inject(Router);
    private readonly commandPalette = inject(CommandPaletteService);
    private readonly ganttOrderApi = inject(GanttOrderApi);
    private readonly toast = inject(ToastNotificationService);
    private readonly destroyRef = inject(DestroyRef);

    // Optimistic manual-order overlay: set on drop, applied over the shared
    // scheduled list until the refreshed server data matches (then auto-cleared).
    private readonly pendingOrder = signal<number[] | null>(null);

    private readonly _wsSub = new Subscription();

    // Toolbar state
    public readonly showFilter$ = this.issueFilterStore.showFilter$;
    public readonly zoomOptions = ZOOM_OPTIONS;
    public readonly cardModeOptions = GANTT_CARD_MODE_OPTIONS;
    public readonly cardMode = signal<IssueCardViewType>(this.loadCardMode());
    public readonly isWbsCollapsed = signal(false);
    public readonly isMinimapVisible = signal(this.loadMinimapPref());

    // Data from service
    public readonly ganttData = toSignal(this.ganttService.data$);

    public readonly scheduledTasks = computed(() =>
        applyPendingOrder(this.ganttData()?.scheduledTasks ?? [], this.pendingOrder())
    );
    public readonly backlogTasks = computed(() => this.ganttData()?.backlogTasks ?? []);

    public readonly backlogHasMore = this.ganttService.backlogHasMore;
    public readonly backlogLoading = this.ganttService.backlogLoading;

    public onLoadMoreBacklog(): void {
        this.ganttService.loadMoreBacklog();
    }

    // Pending relations (optimistic updates)
    private readonly _pendingRelations = signal<GanttRelation[]>([]);

    // Relation the user just drew — its arrow animates in (cleared on API settle
    // so the confirmed re-render doesn't replay the animation)
    public readonly drawInRelation = signal<{ from: number; to: number } | null>(null);

    public readonly relations = computed(() => [
        ...(this.ganttData()?.relations ?? []),
        ...this._pendingRelations()
    ]);
    public readonly columns = computed(() => this.timelineService.getColumns());
    public readonly headerRows = computed(() => this.timelineService.getHeaderRows());
    public readonly totalWidth = computed(() => this.timelineService.getTotalWidth());
    public readonly totalHeight = computed(
        () => this.scheduledTasks().length * this.timelineService.rowHeight()
    );

    // Viewport state (for minimap)
    public readonly viewportScrollLeft = signal<number>(0);
    public readonly viewportWidth = signal<number>(0);

    // Drawing-relation line (source handle → cursor, in canvas coordinates)
    public readonly drawingLine = computed(() => {
        const dragState = this.dragService.state();
        if (dragState.mode !== DragMode.DrawingRelation || !dragState.taskId) return null;
        return {
            sourceTaskId: dragState.taskId,
            sourceSide: dragState.sourceSide ?? HandleSide.Right,
            clientX: dragState.lastClientX,
            clientY: dragState.lastClientY
        };
    });

    // Hover state
    public readonly hoveredTaskId = signal<number | null>(null);

    // Selection state (for keyboard navigation)
    public readonly selectedTaskIndex = signal<number | null>(null);
    public readonly selectedRelationId = signal<number | null>(null);

    // The selected task's public id (derived from index) — consumed by the WBS
    // panel highlight and the command palette context.
    public readonly selectedTaskId = computed(() => {
        const idx = this.selectedTaskIndex();
        if (idx === null) return null;
        return this.scheduledTasks()[idx]?.idIssuePublic ?? null;
    });

    // The selected issue object — fed to the command palette so `>` actions
    // (set state, assign, clone…) target the keyboard-highlighted task.
    public readonly selectedIssue = computed<Issue | null>(() => {
        const idx = this.selectedTaskIndex();
        if (idx === null) return null;
        return this.scheduledTasks()[idx] ?? null;
    });

    // Critical path
    public readonly isCriticalPathEnabled = signal(false);
    public readonly criticalPath = computed(() => {
        if (!this.isCriticalPathEnabled()) return emptyCriticalPath();
        const tasks = this.scheduledTasks();
        const relations = this.relations();
        return this.criticalPathService.computeCriticalPath(tasks, relations);
    });

    // True while the just-enabled critical path "traces" along the chain —
    // limited to a short window so scroll-culling re-renders don't replay it.
    public readonly isCriticalTracing = signal(false);
    private criticalTraceTimer: ReturnType<typeof setTimeout> | null = null;

    // Domino settle after a drop/resize: taskId → stagger delay (ms). While the
    // window is open, bars whose position changes glide (FLIP) instead of jumping;
    // depth in the dependency cascade sets the stagger. Cleared on a timer so
    // unrelated position changes (zoom, later edits) never animate.
    public readonly cascadeSlide = signal<Map<number, number>>(new Map());
    private cascadeSlideTimer: ReturnType<typeof setTimeout> | null = null;

    private startCascadeSlide(movedTaskId: number | null, cascade: CascadeResult): void {
        if (movedTaskId === null && cascade.affectedTasks.size === 0) return;
        const map = new Map<number, number>();
        let depth = 0;
        if (movedTaskId !== null) map.set(movedTaskId, depth++ * 60);
        for (const [taskId] of cascade.affectedTasks) {
            map.set(taskId, depth++ * 60);
        }
        this.cascadeSlide.set(map);
        if (this.cascadeSlideTimer) clearTimeout(this.cascadeSlideTimer);
        this.cascadeSlideTimer = setTimeout(() => this.cascadeSlide.set(new Map()), 2500);
    }

    // Issues by ID for quick actions
    private readonly issueMap = computed(() => {
        const map = new Map<number, Issue>();
        for (const t of [...this.scheduledTasks(), ...this.backlogTasks()]) {
            map.set(t.idIssuePublic, t);
        }
        return map;
    });

    // Cascade preview for drag operations
    public readonly cascadePreview = computed(() => {
        const dragState = this.dragService.state();
        if (dragState.mode !== DragMode.Moving || !dragState.currentDate || !dragState.taskId)
            return null;

        const tasks = this.scheduledTasks();
        const relations = this.relations();
        const task = tasks.find(t => t.idIssuePublic === dragState.taskId);
        if (!task) return null;

        return this.cascadeService.computeCascade(
            dragState.taskId,
            dragState.currentDate,
            task.estimated ?? 3600,
            tasks,
            relations
        );
    });

    // Ghost bars for drag preview
    public readonly ghostBars = computed(() => {
        const dragState = this.dragService.state();
        if (!dragState.taskId || !dragState.currentDate) return [];

        const rowHeight = this.timelineService.rowHeight();
        const isComfortable = isComfortableMode(this.cardMode());
        const minWidth = isComfortable
            ? 2 * BAR_BORDER + 2 * BAR_PADDING_COMFORTABLE
            : 2 * BAR_BORDER + 2 * BAR_PADDING_COMPACT;
        const tooltipText = this.dragService.tooltipText();

        // Backlog scheduling: single ghost at current cursor position, no cascade
        if (dragState.mode === DragMode.SchedulingBacklog) {
            const task = this.backlogTasks().find(t => t.idIssuePublic === dragState.taskId);
            if (!task) return [];
            const estimated = task.estimated ?? 3600;
            const left = this.timelineService.toPixel(dragState.currentDate);
            const right = this.timelineService.toPixel(
                new Date(dragState.currentDate.getTime() + estimated * 1000)
            );
            const rowIdx = this.scheduledTasks().length; // will be appended at end of list
            return [
                {
                    taskId: dragState.taskId,
                    left,
                    top: rowIdx * rowHeight + 2,
                    width: Math.max(minWidth, right - left),
                    tooltipText
                }
            ];
        }

        // Move: use cascade preview for dragged task + dependents
        const preview = this.cascadePreview();
        if (!preview) return [];

        const tasks = this.scheduledTasks();
        const taskIndexMap = new Map(tasks.map((t, i) => [t.idIssuePublic, i]));
        const bars: {
            taskId: number;
            left: number;
            top: number;
            width: number;
            tooltipText: string | null;
        }[] = [];

        const draggedTask = tasks.find(t => t.idIssuePublic === dragState.taskId);
        if (draggedTask) {
            const estimated = draggedTask.estimated ?? 3600;
            const left = this.timelineService.toPixel(dragState.currentDate);
            const right = this.timelineService.toPixel(
                new Date(dragState.currentDate.getTime() + estimated * 1000)
            );
            const rowIdx = taskIndexMap.get(dragState.taskId) ?? 0;
            bars.push({
                taskId: dragState.taskId,
                left,
                top: rowIdx * rowHeight + 2,
                width: Math.max(minWidth, right - left),
                tooltipText
            });
        }

        for (const [taskId, change] of preview.affectedTasks) {
            const task = tasks.find(t => t.idIssuePublic === taskId);
            if (!task) continue;
            const estimated = task.estimated ?? 3600;
            const left = this.timelineService.toPixel(change.scheduledAt);
            const right = this.timelineService.toPixel(
                new Date(change.scheduledAt.getTime() + estimated * 1000)
            );
            const rowIdx = taskIndexMap.get(taskId) ?? 0;
            bars.push({
                taskId,
                left,
                top: rowIdx * rowHeight + 2,
                width: Math.max(minWidth, right - left),
                tooltipText: null
            });
        }

        return bars;
    });

    // Ghost element that follows the cursor when dragging from backlog
    public readonly backlogDragGhost = computed(() => {
        const dragState = this.dragService.state();
        if (dragState.mode !== DragMode.SchedulingBacklog || !dragState.taskId) return null;
        const task = this.backlogTasks().find(t => t.idIssuePublic === dragState.taskId);
        if (!task) return null;
        return {
            title: task.title,
            severityColor: task.severity?.color ?? null,
            x: dragState.lastClientX,
            y: dragState.lastClientY
        };
    });

    // Expose DragMode enum for template
    protected readonly DragMode = DragMode;

    public constructor() {
        effect(() => {
            const completedMode = this.dragService.completed();
            if (completedMode) {
                this.onDragCompleted(completedMode);
            }
        });

        // Recompute timeline range whenever tasks or zoom level change
        effect(() => {
            const tasks = this.scheduledTasks();
            const { start, end } = this.timelineService.computeRange(tasks);
            this.timelineService.setRange(start, end);
        });

        // Clear the optimistic overlay once the refreshed server order (already
        // rank-sorted by the service) matches it — bounds the overlay's lifetime.
        effect(() => {
            const serverIds = (this.ganttData()?.scheduledTasks ?? []).map(t => t.idIssuePublic);
            const pending = this.pendingOrder();
            if (
                pending &&
                serverIds.length === pending.length &&
                serverIds.every((id, i) => id === pending[i])
            ) {
                this.pendingOrder.set(null);
            }
        });

        // Keep the command palette's `>` action target in sync with the
        // keyboard-selected task — same pattern as issue-table.
        effect(() => {
            const issue = this.selectedIssue();
            const idProject = this.scheduledTasks()[0]?.idProject ?? null;
            this.commandPalette.setContext({ idProject, issue });
        });
    }

    public ngAfterViewInit(): void {
        this.issueToolbarService.register(this.toolbarRef());
        this.setInitialFilter();
        this.onSavedViewResetSignal();
        this.updateRowHeight();
        this._wsSub.add(
            this.noticeService.relation$
                .pipe(filter(notice => notice.action === NoticeAction.Create))
                .subscribe(notice => {
                    this.ganttService.addRelations(notice.payload);
                })
        );
        // Live updates: reload the timeline on any issue change and pulse the
        // changed bars once the refreshed data renders. Own drops pulse too —
        // the ring doubles as the "change persisted" confirmation (a bulk-edit
        // cascade sends one notice per affected task, so all of them pulse).
        this._wsSub.add(
            this.noticeService.issue$.subscribe(notice => {
                const idIssuePublic = notice.payload?.idIssuePublic;
                if (idIssuePublic != null) {
                    this.pendingPulseIds.add(idIssuePublic);
                    this.pendingPulseAt = Date.now();
                }
                this.issueFilterStore.refresh();
            })
        );
    }

    // Bars to pulse once the refresh after issue notices re-renders them.
    private readonly pendingPulseIds = new Set<number>();
    private pendingPulseAt = 0;

    private readonly pulseOnDataArrival = effect(() => {
        this.scheduledTasks();
        if (this.pendingPulseIds.size === 0) return;
        const ids = [...this.pendingPulseIds];
        this.pendingPulseIds.clear();
        if (Date.now() - this.pendingPulseAt > 4000) return;
        requestAnimationFrame(() => {
            for (const idIssuePublic of ids) {
                const barEl = document.querySelector<HTMLElement>(
                    `.gantt-bar[data-task-id="${idIssuePublic}"]`
                );
                if (barEl) pulseElement(barEl);
            }
        });
    });

    public ngOnDestroy(): void {
        this._wsSub.unsubscribe();
        this.issueToolbarService.clear();
        this.commandPalette.setContext({ idProject: null, issue: null });
    }

    // --- Toolbar handlers ---

    public onToggleFilter(): void {
        this.issueFilterStore.toggleShowFilter();
    }

    public onZoomChange(level: GanttZoomLevel): void {
        // A zoom re-positions every bar; never let it replay the post-drop slide
        this.cascadeSlide.set(new Map());
        this.timelineService.setZoom(level);
    }

    public onCardModeChange(mode: IssueCardViewType): void {
        this.cardMode.set(mode);
        localStorage.setItem(STORAGE_KEY_CARD_MODE, mode);
        this.updateRowHeight();
    }

    public onToggleWbs(): void {
        this.isWbsCollapsed.set(!this.isWbsCollapsed());
    }

    public onToggleMinimap(): void {
        const next = !this.isMinimapVisible();
        this.isMinimapVisible.set(next);
        localStorage.setItem(STORAGE_KEY_MINIMAP, String(next));
        if (next) {
            const container = this.timelineBodyRef()?.getScrollContainer();
            if (container) {
                this.viewportScrollLeft.set(container.scrollLeft);
                this.viewportWidth.set(container.clientWidth);
            }
        }
    }

    public onScrollToToday(): void {
        this.timelineBodyRef()?.scrollToToday();
    }

    public onToggleCriticalPath(): void {
        const isEnabling = !this.isCriticalPathEnabled();
        this.isCriticalPathEnabled.set(isEnabling);
        if (this.criticalTraceTimer) clearTimeout(this.criticalTraceTimer);
        if (isEnabling) {
            // Trace window: per-segment stagger plus the draw duration
            const segments = this.criticalPath().relationOrder.size;
            this.isCriticalTracing.set(true);
            this.criticalTraceTimer = setTimeout(
                () => this.isCriticalTracing.set(false),
                segments * 120 + 700
            );
        } else {
            this.isCriticalTracing.set(false);
        }
    }

    // --- Scroll sync ---

    public onWbsScrolled(scrollTop: number): void {
        this.timelineBodyRef()?.syncScrollFrom(scrollTop);
    }

    public onTimelineScrolled(event: { scrollTop: number; scrollLeft: number }): void {
        this.wbsPanelRef()?.syncScrollFrom(event.scrollTop);
        this.viewportScrollLeft.set(event.scrollLeft);
        const container = this.timelineBodyRef()?.getScrollContainer();
        if (container) this.viewportWidth.set(container.clientWidth);
    }

    // --- Hover sync ---

    public onTaskHovered(taskId: number | null): void {
        this.hoveredTaskId.set(taskId);
    }

    // --- Context menu ---

    public onBarContextMenu(event: { taskId: number; event: MouseEvent }): void {
        const issue = this.issueMap().get(event.taskId);
        if (!issue) return;
        this.quickActionsRef()?.show(event.event, issue);
    }

    // --- Drag handlers ---

    public onBarDragStarted(event: { taskId: number; event: MouseEvent }): void {
        const task = this.issueMap().get(event.taskId);
        if (!task?.scheduledAt) return;
        this.dragService.startMove(event.taskId, event.event.clientX, task.scheduledAt);
    }

    public onBarResizeEnded(event: { taskId: number; newEstimated: number }): void {
        const task = this.issueMap().get(event.taskId);
        if (!task?.scheduledAt) return;

        const cascade = this.cascadeService.computeCascade(
            event.taskId,
            task.scheduledAt,
            event.newEstimated,
            this.scheduledTasks(),
            this.relations()
        );

        const entries: BulkEditIssueEntry[] = [
            { idIssuePublic: event.taskId, estimated: event.newEstimated }
        ];
        for (const [taskId, change] of cascade.affectedTasks) {
            entries.push({ idIssuePublic: taskId, scheduledAt: change.scheduledAt.toISOString() });
        }
        this.startCascadeSlide(null, cascade);

        this.bulkApi.update$(task.idProject, entries).subscribe({
            next: () => this.issueFilterStore.refresh(),
            error: () => this.issueFilterStore.refresh()
        });
    }

    public onScheduledReorder(evt: { movedId: number; order: number[] }): void {
        const idProject = this.scheduledTasks()[0]?.idProject;
        if (!idProject) return;
        this.pendingOrder.set(evt.order); // synchronous → no snap-back frame
        this.ganttOrderApi.reorder$(idProject, evt).subscribe({
            next: () => this.issueFilterStore.refresh(), // refreshed rank order == overlay → effect clears it
            error: () => {
                this.pendingOrder.set(null); // roll back the optimistic move
                this.issueFilterStore.refresh();
                this.toast.showError('ISSUE.MOVE_FAILED');
            }
        });
    }

    public onBacklogDragStarted(event: { taskId: number; event: MouseEvent }): void {
        const container = this.timelineBodyRef()?.getScrollContainer();
        if (container) {
            const rect = container.getBoundingClientRect();
            this.dragService.setCanvasOffset(rect.left - container.scrollLeft);
        }
        this.dragService.startBacklogSchedule(event.taskId, event.event.clientX);
    }

    public onConnectionHandleDragStarted(event: {
        taskId: number;
        side: HandleSide;
        event: MouseEvent;
    }): void {
        this.dragService.startRelationDraw(
            event.taskId,
            event.side,
            event.event.clientX,
            event.event.clientY
        );
    }

    public onDragCompleted(completedMode: DragMode): void {
        const dragState = this.dragService.state();

        if (completedMode === DragMode.Moving) {
            const moveResult = this.dragService.getMoveDelta();
            if (!moveResult || !dragState.taskId) {
                this.dragService.reset();
                return;
            }

            const tasks = this.scheduledTasks();
            const cascade = this.cascadeService.computeCascade(
                dragState.taskId,
                moveResult.newScheduledAt,
                tasks.find(t => t.idIssuePublic === dragState.taskId)?.estimated ?? 3600,
                tasks,
                this.relations()
            );

            const entries: BulkEditIssueEntry[] = [
                {
                    idIssuePublic: dragState.taskId,
                    scheduledAt: moveResult.newScheduledAt.toISOString()
                }
            ];
            for (const [taskId, change] of cascade.affectedTasks) {
                entries.push({
                    idIssuePublic: taskId,
                    scheduledAt: change.scheduledAt.toISOString()
                });
            }

            const idProject = tasks[0]?.idProject;
            if (!idProject) {
                this.dragService.reset();
                return;
            }

            this.startCascadeSlide(dragState.taskId, cascade);
            this.bulkApi.update$(idProject, entries).subscribe({
                next: () => {
                    this.issueFilterStore.refresh();
                    this.dragService.reset();
                },
                error: () => {
                    this.issueFilterStore.refresh();
                    this.dragService.reset();
                }
            });
            return;
        }

        // DragMode.Resizing is handled directly in onBarResizeEnded (task bar emits final estimated)

        if (completedMode === DragMode.SchedulingBacklog && dragState.taskId) {
            const result = this.dragService.getBacklogScheduleResult();
            if (!result) {
                this.dragService.reset();
                return;
            }

            // Only schedule if the drop landed inside the timeline canvas
            const container = this.timelineBodyRef()?.getScrollContainer();
            if (container) {
                const rect = container.getBoundingClientRect();
                const { lastClientX: x, lastClientY: y } = dragState;
                if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) {
                    this.dragService.reset();
                    return;
                }
            }

            const task = this.issueMap().get(dragState.taskId);
            if (!task) {
                this.dragService.reset();
                return;
            }

            const updated = {
                ...task,
                scheduledAt: result.scheduledAt,
                estimated: result.estimated
            };
            this.issueService.updateIssue(updated).subscribe({
                next: () => {
                    this.issueFilterStore.refresh();
                    this.dragService.reset();
                },
                error: () => {
                    this.issueFilterStore.refresh();
                    this.dragService.reset();
                }
            });
            return;
        }

        if (completedMode === DragMode.DrawingRelation && dragState.taskId) {
            const dropTarget = this.dragService.lastDropTarget();
            if (!dropTarget || dropTarget.taskId === dragState.taskId) {
                this.dragService.reset();
                return;
            }
            this.onRelationCreated(
                dragState.taskId,
                dragState.sourceSide ?? HandleSide.Right,
                dropTarget.taskId,
                dropTarget.side
            );
            this.dragService.reset();
        }
    }

    // --- Keyboard bindings ---

    @HostListener('window:keydown', ['$event'])
    public onKeyDown(event: KeyboardEvent): void {
        // Defer to the command palette OR the shortcuts help sheet while either is open,
        // so their keys aren't stolen by the Gantt's global window:keydown (design §hard gate).
        if (this.commandPalette.isOverlayOpen()) return;
        // Never steal keys from text inputs (filter fields etc.)
        const target = event.target as HTMLElement | null;
        if (target?.closest('input, textarea, select, [contenteditable]')) return;
        switch (event.key) {
            case 't':
            case 'Home':
                this.timelineBodyRef()?.scrollToToday();
                break;
            case '+':
            case '=':
                this.zoomStep(-1);
                break;
            case '-':
                this.zoomStep(1);
                break;
            case 'ArrowUp': {
                event.preventDefault();
                const current = this.selectedTaskIndex() ?? 0;
                const next = Math.max(0, current - 1);
                this.selectedTaskIndex.set(next);
                break;
            }
            case 'ArrowDown': {
                event.preventDefault();
                const tasks = this.scheduledTasks();
                const current = this.selectedTaskIndex() ?? -1;
                const next = Math.min(tasks.length - 1, current + 1);
                this.selectedTaskIndex.set(next);
                break;
            }
            case 'ArrowLeft':
                this.timelineBodyRef()?.panHorizontal(-100);
                break;
            case 'ArrowRight':
                this.timelineBodyRef()?.panHorizontal(100);
                break;
            case 'Enter': {
                const idx = this.selectedTaskIndex();
                if (idx !== null) {
                    const task = this.scheduledTasks()[idx];
                    if (task) {
                        void this.router.navigate([
                            '/project',
                            task.idProject,
                            'issue',
                            task.idIssuePublic
                        ]);
                    }
                }
                break;
            }
            case 'Delete':
            case 'Backspace': {
                const relId = this.selectedRelationId();
                if (relId !== null) {
                    const relation = this.relations().find(
                        r =>
                            r.idIssueRelation === relId &&
                            r.direction === IssueRelationDirection.Outbound
                    );
                    if (relation) {
                        const idProject = this.scheduledTasks()[0]?.idProject;
                        if (idProject) {
                            this.onDeleteRelation({
                                relationId: relId,
                                idProject,
                                idIssuePublic: relation.from.idIssuePublic
                            });
                        }
                    }
                }
                break;
            }
            case 'Escape':
                if (this.dragService.isDragging()) {
                    this.dragService.cancel();
                }
                this.selectedTaskIndex.set(null);
                this.selectedRelationId.set(null);
                break;
        }
    }

    @HostListener('wheel', ['$event'])
    public onWheel(event: WheelEvent): void {
        if (!event.ctrlKey) return;
        event.preventDefault();

        // Cursor-anchored zoom: preserve the date under the cursor
        const container = this.timelineBodyRef()?.getScrollContainer();
        if (container) {
            const rect = container.getBoundingClientRect();
            const cursorX = event.clientX - rect.left + container.scrollLeft;
            const dateUnderCursor = this.timelineService.toDate(cursorX);

            this.zoomStep(event.deltaY > 0 ? 1 : -1);

            // After zoom, recompute pixel position of the same date and adjust scroll
            const newPixel = this.timelineService.toPixel(dateUnderCursor);
            const cursorOffset = event.clientX - rect.left;
            container.scrollLeft = newPixel - cursorOffset;
        } else {
            this.zoomStep(event.deltaY > 0 ? 1 : -1);
        }
    }

    // --- WBS task click ---

    public onWbsTaskClicked(event: { taskId: number; isBacklog: boolean }): void {
        if (!event.isBacklog) {
            const task = this.scheduledTasks().find(t => t.idIssuePublic === event.taskId);
            if (task?.scheduledAt) {
                const px = this.timelineService.toPixel(task.scheduledAt);
                this.timelineBodyRef()?.scrollToPixel(px);
            }
        }
    }

    // --- Private helpers ---

    private zoomStep(direction: number): void {
        const current = this.timelineService.zoomLevel();
        const idx = ZOOM_LEVELS.indexOf(current);
        const next = idx + direction;
        if (next < 0 || next >= ZOOM_LEVELS.length) return;
        this.timelineService.setZoom(ZOOM_LEVELS[next]);
    }

    private updateRowHeight(): void {
        const height = isComfortableMode(this.cardMode())
            ? ROW_HEIGHT_COMFORTABLE
            : ROW_HEIGHT_COMPACT;
        this.timelineService.rowHeight.set(height);
    }

    private onSavedViewResetSignal(): void {
        this.savedViewStore.filterResetSignal$
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.setInitialFilter());
    }

    private setInitialFilter(): void {
        this.projectStore.project$.pipe(first()).subscribe(project => {
            const now = new Date();
            // Never persisted in a view, so both branches need it computed here.
            const scheduledAtFrom = startOfMonth(subMonths(now, 1));
            const scheduledAtTo = endOfMonth(addMonths(now, 2));
            const pending = this.savedViewStore.consumePending(project.idProject);
            if (pending) {
                this.issueFilterStore.setInitialFilter({
                    ...SavedViewConfigConverter.toFilter(pending.config),
                    scheduledAtFrom,
                    scheduledAtTo,
                    idProject: project.idProject
                });
                return;
            }
            this.issueFilterStore.setInitialFilter({
                idProject: project.idProject,
                stateUnset: true,
                idsState: [],
                idsSeverity: [],
                severityUnset: true,
                assignedToUnset: true,
                idsAssignedTo: [],
                orderColumn: 'scheduledAt',
                orderDirection: 'asc',
                scheduledAtFrom,
                scheduledAtTo
            });
        });
    }

    private loadCardMode(): IssueCardViewType {
        const stored = localStorage.getItem(STORAGE_KEY_CARD_MODE);
        return stored === 'GanttCompact' ? 'GanttCompact' : 'GanttComfort';
    }

    private loadMinimapPref(): boolean {
        return localStorage.getItem(STORAGE_KEY_MINIMAP) !== 'false';
    }

    public onDeleteRelation(event: {
        relationId: number;
        idProject: number;
        idIssuePublic: number;
    }): void {
        this.ganttService.removeRelation(event.relationId);
        this.selectedRelationId.set(null);
        this.relationApi.delete$(event.idProject, event.idIssuePublic, event.relationId).subscribe({
            error: () => this.issueFilterStore.refresh()
        });
    }

    // --- Relation creation helpers ---

    private inferRelationSubType(
        sourceSide: HandleSide,
        targetSide: HandleSide
    ): IssueRelationSubType {
        if (sourceSide === HandleSide.Right && targetSide === HandleSide.Left)
            return IssueRelationSubType.FinishToStart;
        if (sourceSide === HandleSide.Left && targetSide === HandleSide.Left)
            return IssueRelationSubType.StartToStart;
        if (sourceSide === HandleSide.Right && targetSide === HandleSide.Right)
            return IssueRelationSubType.FinishToFinish;
        if (sourceSide === HandleSide.Left && targetSide === HandleSide.Right)
            return IssueRelationSubType.StartToFinish;
        return IssueRelationSubType.FinishToStart;
    }

    private wouldCreateCycle(sourceId: number, targetId: number): boolean {
        // BFS from targetId through outbound schedule relations
        // If we reach sourceId, it's a cycle
        const relations = this.relations();
        const adjacency = new Map<number, number[]>();
        for (const r of relations) {
            if (
                r.direction !== IssueRelationDirection.Outbound ||
                r.relationType !== IssueRelationType.Schedule
            )
                continue;
            if (!adjacency.has(r.from.idIssuePublic)) adjacency.set(r.from.idIssuePublic, []);
            adjacency.get(r.from.idIssuePublic)!.push(r.to.idIssuePublic);
        }
        // Add the proposed edge
        if (!adjacency.has(sourceId)) adjacency.set(sourceId, []);
        adjacency.get(sourceId)!.push(targetId);

        const visited = new Set<number>();
        const queue = [targetId];
        while (queue.length > 0) {
            const current = queue.shift()!;
            if (current === sourceId) return true;
            if (visited.has(current)) continue;
            visited.add(current);
            for (const neighbor of adjacency.get(current) ?? []) {
                queue.push(neighbor);
            }
        }
        return false;
    }

    private onRelationCreated(
        sourceId: number,
        sourceSide: HandleSide,
        targetId: number,
        targetSide: HandleSide
    ): void {
        if (sourceId === targetId) return;
        if (this.wouldCreateCycle(sourceId, targetId)) return;

        const idProject = this.scheduledTasks()[0]?.idProject;
        if (!idProject) return;

        const subType = this.inferRelationSubType(sourceSide, targetSide);
        const tempId = -Date.now();
        const pendingRelation: GanttRelation = {
            idIssueRelation: tempId,
            relationType: IssueRelationType.Schedule,
            relationSubType: subType,
            lagMinutes: null,
            direction: IssueRelationDirection.Outbound,
            label: '',
            inverseLabel: '',
            from: { idIssuePublic: sourceId },
            to: { idIssuePublic: targetId },
            createdAt: new Date().toISOString(),
            createdBy: 0
        };
        this._pendingRelations.update(p => [...p, pendingRelation]);

        // Hold the draw-in flag for the full animation duration — clearing it on
        // API confirm cuts the animation short (locally the API answers in ~20ms).
        this.drawInRelation.set({ from: sourceId, to: targetId });
        setTimeout(() => {
            const current = this.drawInRelation();
            if (current?.from === sourceId && current?.to === targetId) {
                this.drawInRelation.set(null);
            }
        }, 800);

        this.relationApi
            .insert$(idProject, sourceId, {
                idIssuePublicTo: targetId,
                relationType: IssueRelationType.Schedule,
                relationSubType: subType
            })
            .subscribe({
                next: confirmedRelations => {
                    this._pendingRelations.update(p => p.filter(r => r.idIssueRelation !== tempId));
                    this.ganttService.addRelations(confirmedRelations);
                },
                error: () => {
                    this.drawInRelation.set(null);
                    this._pendingRelations.update(p => p.filter(r => r.idIssueRelation !== tempId));
                }
            });
    }

    protected readonly isSplitDialogOpen = signal(false);

    protected readonly splitIssue = signal<Issue | null>(null);

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
}
