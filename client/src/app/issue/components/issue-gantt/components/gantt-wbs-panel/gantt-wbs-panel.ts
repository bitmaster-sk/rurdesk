import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    computed,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { GanttTimelineService } from '../../service/gantt-timeline.service';
import { ExtendedIssue, ScheduledIssue } from '../../../../model/extended-issue.model';
import { STORAGE_KEY_WBS_WIDTH } from '../../constants/gantt-storage-keys';
import {
    DEFAULT_WIDTH,
    DROP_TIP_GAP_PX,
    MAX_WIDTH,
    MIN_WIDTH,
    OUTLINE_DRAG_PX
} from './constants/gantt-wbs-panel.constants';
import { GanttTreeRow, ParentMap } from '../../entity/gantt-tree.entity';
import { GanttWbsDrop, GanttWbsDropKind } from '../../entity/gantt-wbs-drop.entity';
import { GanttRowDragMove, GanttWbsDropTarget } from './entity/gantt-wbs-panel.entity';
import { GanttTreeConverter } from '../../converter/gantt-tree.converter';

@Component({
    selector: 'app-gantt-wbs-panel',
    templateUrl: './gantt-wbs-panel.html',
    styleUrl: './gantt-wbs-panel.scss',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class GanttWbsPanelComponent {
    public readonly timelineService = inject(GanttTimelineService);

    private readonly scrollContainer = viewChild<ElementRef<HTMLDivElement>>('scrollContainer');

    public readonly rows = input.required<GanttTreeRow<ScheduledIssue>[]>();
    public readonly parents = input<ParentMap>(new Map());
    public readonly backlogTasks = input.required<ExtendedIssue[]>();
    public readonly backlogHasMore = input<boolean>(false);
    public readonly backlogLoading = input<boolean>(false);
    public readonly isCollapsed = input<boolean>(false);
    public readonly hoveredTaskId = input<number | null>(null);
    public readonly selectedTaskId = input<number | null>(null);
    public readonly hasHierarchy = input<boolean>(false);
    public readonly outlineLevel = input<number | null>(0);

    public readonly scrolled = output<number>();
    public readonly taskClicked = output<{
        taskId: number;
        isBacklog: boolean;
    }>();
    public readonly taskHovered = output<number | null>();
    public readonly backlogDragStarted = output<{
        taskId: number;
        event: MouseEvent;
    }>();
    public readonly loadMoreBacklog = output<void>();
    public readonly treeDropped = output<GanttWbsDrop>();
    public readonly collapseToggled = output<number>();
    public readonly outlineLevelChange = output<number>();

    public readonly outlineOptions = [
        { label: '1', value: 1 },
        { label: '2', value: 2 },
        { label: '3', value: 3 },
        { label: 'ISSUE.GANTT.OUTLINE_ALL', value: 0 }
    ];

    public readonly panelWidth = signal<number>(this.loadWidth());
    public readonly activeTab = signal<'scheduled' | 'backlog'>('scheduled');
    private isResizing = false;
    private isSyncingScroll = false;
    private isDragging = false;

    public readonly dropTarget = signal<GanttWbsDropTarget | null>(null);
    public readonly dropTipAt = signal<{ x: number; y: number } | null>(null);
    private draggedId: number | null = null;

    public readonly hasNoIssues = computed(
        () => this.rows().length === 0 && this.backlogTasks().length === 0
    );

    public readonly rowHeight = computed(() => this.timelineService.rowHeight());

    public readonly newSummary = computed(() => {
        const drop = this.dropTarget();
        if (!drop?.isAllowed) return null;
        const order = this.rows().map(r => r.task.idIssuePublic);
        const idParent =
            drop.kind === 'into'
                ? drop.targetId
                : drop.kind === 'indent'
                  ? GanttTreeConverter.indentTarget(drop.movedId, order, this.parents())
                  : null;
        const parent = this.rows().find(r => r.task.idIssuePublic === idParent)?.task;
        if (!parent || parent.isSummary || !(parent.estimated ?? 0)) return null;
        return parent;
    });

    public onScroll(): void {
        if (this.isSyncingScroll) {
            this.isSyncingScroll = false;
            return;
        }
        const el = this.scrollContainer()?.nativeElement;
        if (!el) return;
        this.scrolled.emit(el.scrollTop);
    }

    public syncScrollFrom(scrollTop: number): void {
        const el = this.scrollContainer()?.nativeElement;
        if (!el) return;
        this.isSyncingScroll = true;
        el.scrollTop = scrollTop;
    }

    public onResizeStart(event: MouseEvent): void {
        event.preventDefault();
        this.isResizing = true;
        const startX = event.clientX;
        const startWidth = this.panelWidth();

        const onMouseMove = (moveEvent: MouseEvent): void => {
            const delta = moveEvent.clientX - startX;
            const newWidth = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth + delta));
            this.panelWidth.set(newWidth);
        };

        const onMouseUp = (): void => {
            this.isResizing = false;
            localStorage.setItem(STORAGE_KEY_WBS_WIDTH, String(this.panelWidth()));
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);
        };

        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', onMouseUp);
    }

    public onTaskClick(taskId: number, isBacklog: boolean): void {
        if (this.isDragging) return;
        this.taskClicked.emit({ taskId, isBacklog });
    }

    public onToggleCollapse(event: Event, taskId: number): void {
        event.stopPropagation();
        this.collapseToggled.emit(taskId);
    }

    public static resolveDrop(
        movedId: number,
        targetId: number,
        fraction: number,
        dx: number,
        parents: ParentMap,
        order: number[]
    ): GanttWbsDropTarget {
        if (targetId === movedId) {
            const kind: GanttWbsDropKind | null =
                dx >= OUTLINE_DRAG_PX ? 'indent' : dx <= -OUTLINE_DRAG_PX ? 'outdent' : null;
            const isAllowed =
                kind === 'indent'
                    ? GanttTreeConverter.indentTarget(movedId, order, parents) !== null
                    : kind === 'outdent' && parents.has(movedId);
            return { movedId, targetId, kind: kind ?? 'into', isAllowed };
        }
        return {
            movedId,
            targetId,
            kind: GanttTreeConverter.dropZone(fraction),
            isAllowed: GanttTreeConverter.canDrop(movedId, targetId, parents)
        };
    }

    public onRowDragStarted(taskId: number): void {
        this.isDragging = true;
        this.draggedId = taskId;
    }

    public onRowDragMoved(event: GanttRowDragMove): void {
        if (this.draggedId === null) return;
        const { x, y } = event.pointerPosition;
        const placeholder = event.source.getPlaceholderElement();
        const own = placeholder.getBoundingClientRect();
        this.dropTipAt.set({ x, y: own.bottom + event.distance.y + DROP_TIP_GAP_PX });
        const overOwnRow = y >= own.top && y < own.bottom;
        // The CDK placeholder has pointer-events: none, so elementsFromPoint never returns the own row.
        const rowEl = overOwnRow
            ? placeholder
            : document
                  .elementsFromPoint(x, y)
                  .find(
                      (el): el is HTMLElement =>
                          el instanceof HTMLElement &&
                          el.classList.contains('wbs-row') &&
                          !el.classList.contains('cdk-drag-preview') &&
                          el.dataset['taskId'] !== undefined
                  );
        if (!rowEl) {
            this.dropTarget.set(null);
            return;
        }
        const targetId = Number(rowEl.dataset['taskId']);
        const isContext = rowEl.classList.contains('wbs-row--context');
        const rect = rowEl.getBoundingClientRect();
        const target = GanttWbsPanelComponent.resolveDrop(
            this.draggedId,
            targetId,
            (y - rect.top) / rect.height,
            event.distance.x,
            this.parents(),
            this.rows().map(r => r.task.idIssuePublic)
        );
        this.dropTarget.set(isContext ? { ...target, isAllowed: false } : target);
    }

    public onScheduledDrop(): void {
        const target = this.dropTarget();
        this.dropTarget.set(null);
        this.dropTipAt.set(null);
        this.draggedId = null;
        if (target?.isAllowed) {
            this.treeDropped.emit({
                movedId: target.movedId,
                targetId: target.targetId,
                kind: target.kind
            });
        }
    }

    public onRowDragEnded(): void {
        this.dropTipAt.set(null);
        // Clear on the next tick so the click CDK synthesizes right after the drop
        // is still suppressed by onTaskClick.
        setTimeout(() => (this.isDragging = false));
    }

    public dropLabelKey(target: GanttWbsDropTarget): string {
        if (!target.isAllowed) return 'ISSUE.GANTT.DROP_NOT_ALLOWED';
        switch (target.kind) {
            case 'before':
                return 'ISSUE.GANTT.DROP_BEFORE';
            case 'after':
                return 'ISSUE.GANTT.DROP_AFTER';
            case 'indent':
                return 'ISSUE.GANTT.INDENT';
            case 'outdent':
                return 'ISSUE.GANTT.OUTDENT';
            default:
                return 'ISSUE.GANTT.DROP_INTO';
        }
    }

    public titleOf(taskId: number): string {
        return this.rows().find(r => r.task.idIssuePublic === taskId)?.task.title ?? '';
    }

    public onTaskMouseEnter(taskId: number): void {
        this.taskHovered.emit(taskId);
    }

    public onTaskMouseLeave(): void {
        this.taskHovered.emit(null);
    }

    public onBacklogDragStart(taskId: number, event: MouseEvent): void {
        if (event.button !== 0) return;
        event.preventDefault();
        // Use a drag threshold to distinguish click from drag
        const startX = event.clientX;
        const startY = event.clientY;
        const DRAG_THRESHOLD = 5;

        const onMouseMove = (moveEvent: MouseEvent): void => {
            const dx = moveEvent.clientX - startX;
            const dy = moveEvent.clientY - startY;
            if (Math.abs(dx) > DRAG_THRESHOLD || Math.abs(dy) > DRAG_THRESHOLD) {
                document.removeEventListener('mousemove', onMouseMove);
                document.removeEventListener('mouseup', onMouseUp);
                this.backlogDragStarted.emit({ taskId, event });
            }
        };

        const onMouseUp = (): void => {
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);
            // Threshold not exceeded — treat as click
            this.taskClicked.emit({ taskId, isBacklog: true });
        };

        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', onMouseUp);
    }

    private loadWidth(): number {
        const stored = localStorage.getItem(STORAGE_KEY_WBS_WIDTH);
        const parsed = stored ? parseInt(stored, 10) : NaN;
        return isNaN(parsed) ? DEFAULT_WIDTH : Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, parsed));
    }
}
