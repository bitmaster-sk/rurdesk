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
import { CdkDragDrop } from '@angular/cdk/drag-drop';
import { GanttTimelineService } from '../../service/gantt-timeline.service';
import { ExtendedIssue, ScheduledIssue } from '../../../../model/extended-issue.model';
import { STORAGE_KEY_WBS_WIDTH } from '../../constants/gantt-storage-keys';

const DEFAULT_WIDTH = 280;
const MIN_WIDTH = 180;
const MAX_WIDTH = 600;

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

    public readonly scheduledTasks = input.required<ScheduledIssue[]>();
    public readonly backlogTasks = input.required<ExtendedIssue[]>();
    public readonly backlogHasMore = input<boolean>(false);
    public readonly backlogLoading = input<boolean>(false);
    public readonly isCollapsed = input<boolean>(false);
    public readonly hoveredTaskId = input<number | null>(null);
    public readonly selectedTaskId = input<number | null>(null);

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
    public readonly reordered = output<{ movedId: number; order: number[] }>();

    public readonly panelWidth = signal<number>(this.loadWidth());
    public readonly activeTab = signal<'scheduled' | 'backlog'>('scheduled');
    private isResizing = false;
    private isSyncingScroll = false;
    private isDragging = false;

    public readonly hasNoIssues = computed(
        () => this.scheduledTasks().length === 0 && this.backlogTasks().length === 0
    );

    public readonly rowHeight = computed(() => this.timelineService.rowHeight());

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

    public static buildReorder(
        idsBefore: number[],
        previousIndex: number,
        currentIndex: number
    ): { movedId: number; order: number[] } {
        const order = [...idsBefore];
        const [movedId] = order.splice(previousIndex, 1);
        order.splice(currentIndex, 0, movedId);
        return { movedId, order };
    }

    public onScheduledDrop(event: CdkDragDrop<unknown>): void {
        if (event.previousIndex === event.currentIndex) return;
        const ids = this.scheduledTasks().map(t => t.idIssuePublic);
        this.reordered.emit(
            GanttWbsPanelComponent.buildReorder(ids, event.previousIndex, event.currentIndex)
        );
    }

    public onRowDragStarted(): void {
        this.isDragging = true;
    }

    public onRowDragEnded(): void {
        // Clear on the next tick so the click CDK synthesizes right after the drop
        // is still suppressed by onTaskClick.
        setTimeout(() => (this.isDragging = false));
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
