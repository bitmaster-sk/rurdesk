import {
    AfterViewChecked,
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    NgZone,
    computed,
    effect,
    inject,
    input,
    output,
    signal,
    untracked
} from '@angular/core';
import { Router } from '@angular/router';
import { ScheduledIssue } from '../../../../model/extended-issue.model';
import { GanttTimelineService } from '../../service/gantt-timeline.service';
import { HandleSide } from '../../constants/gantt-handle-side.enum';
import {
    IssueCardViewType,
    isComfortableMode
} from '../../../../constants/issue-card-view-type.constant';
import { addSeconds } from 'date-fns';
import { HORIZONTAL_OFFSET } from '../../service/gantt-arrow-routing';
import {
    prefersReducedMotion,
    UI_SETTLE_DURATION_MS,
    UI_SETTLE_EASING
} from 'src/app/ui/util/motion';

export const MIN_BAR_WIDTH_PX = 4;
export const SMALL_BAR_THRESHOLD_PX = 24;
const BAR_GAP = 12;
export const BAR_BORDER = 1;
export const BAR_PADDING_COMFORTABLE = 8;
export const BAR_PADDING_COMPACT = 6;
// Must exceed HORIZONTAL_OFFSET so the label clears the arrow stub extending past the bar edge
const OVERFLOW_LABEL_GAP = HORIZONTAL_OFFSET + 4;

@Component({
    selector: 'app-gantt-task-bar',
    templateUrl: './gantt-task-bar.html',
    styleUrl: './gantt-task-bar.scss',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class GanttTaskBarComponent implements AfterViewChecked {
    private readonly router = inject(Router);
    public readonly timelineService = inject(GanttTimelineService);
    private readonly ngZone = inject(NgZone);
    private readonly elementRef = inject<ElementRef<HTMLElement>>(ElementRef);

    protected readonly HandleSide = HandleSide;

    public readonly task = input.required<ScheduledIssue>();
    public readonly cardMode = input.required<IssueCardViewType>();
    public readonly rowIndex = input.required<number>();
    public readonly isOnCriticalPath = input<boolean>(false);
    public readonly isDimmed = input<boolean>(false);
    public readonly isSelected = input<boolean>(false);
    /** Side of this bar the in-progress relation draw would connect to, if any. */
    public readonly relationDropSide = input<HandleSide | null>(null);
    /** Stagger delay for the post-drop domino settle; null = don't animate moves. */
    public readonly cascadeSlideDelayMs = input<number | null>(null);

    public readonly hovered = output<number | null>();
    public readonly dragStarted = output<{ taskId: number; event: MouseEvent }>();
    public readonly resizeEnded = output<{ taskId: number; newEstimated: number }>();
    public readonly contextMenu = output<{ taskId: number; event: MouseEvent }>();
    public readonly connectionDragStarted = output<{
        taskId: number;
        side: HandleSide;
        event: MouseEvent;
    }>();

    // Live resize preview — null means use task data width. Held at the snapped
    // width after mouseup and cleared only when refreshed task data arrives, so
    // the bar doesn't flash back to its pre-resize width while the API round-trip
    // is in flight.
    private readonly _resizePreviewWidth = signal<number | null>(null);
    // Suppresses the click event that fires after a drag completes
    private _dragMoved = false;

    public constructor() {
        effect(() => {
            this.task();
            untracked(() => this._resizePreviewWidth.set(null));
        });
    }

    // FLIP settle: while the cascade-slide window is open, a bar whose rendered
    // left changed glides from its old position (delayed by cascade depth)
    // instead of jumping. Runs in ngAfterViewChecked so the animation starts in
    // the same frame the new `left` is painted — no flash at the new position.
    private previousBarLeft: number | null = null;

    public ngAfterViewChecked(): void {
        const left = this.barLeft();
        const previous = this.previousBarLeft;
        this.previousBarLeft = left;

        const delay = this.cascadeSlideDelayMs();
        if (previous === null || delay === null || previous === left) return;
        if (prefersReducedMotion()) return;

        const barEl = this.elementRef.nativeElement.querySelector(
            '.gantt-bar'
        ) as HTMLElement | null;
        if (!barEl) return;
        // A slide may still be in flight from a previous drop — replace it
        barEl.getAnimations().forEach(animation => animation.cancel());
        barEl.animate(
            [{ transform: `translateX(${previous - left}px)` }, { transform: 'translateX(0)' }],
            {
                duration: UI_SETTLE_DURATION_MS,
                delay,
                easing: UI_SETTLE_EASING,
                // Hold the old position through the delay — that's the domino
                fill: 'backwards'
            }
        );
    }

    public readonly barLeft = computed(() => this.timelineService.toPixel(this.task().scheduledAt));

    public readonly barWidth = computed(() => {
        const preview = this._resizePreviewWidth();
        if (preview !== null) return preview;

        const task = this.task();
        const endDate = addSeconds(task.scheduledAt, task.estimated ?? 3600);
        const endPixel = this.timelineService.toPixel(endDate);
        return Math.max(MIN_BAR_WIDTH_PX, endPixel - this.barLeft());
    });

    public readonly barTop = computed(() => {
        const rowHeight = this.timelineService.rowHeight();
        return this.rowIndex() * rowHeight + BAR_GAP / 2;
    });

    public readonly barHeight = computed(() => this.timelineService.rowHeight() - BAR_GAP);

    public readonly isSmallBar = computed(() => this.barWidth() < SMALL_BAR_THRESHOLD_PX);

    public readonly isComfortable = computed(() => isComfortableMode(this.cardMode()));

    public readonly progress = computed(() => {
        const issue = this.task();
        return issue.estimated
            ? Math.min(100, Math.round((issue.tracked / issue.estimated) * 100))
            : 0;
    });

    public readonly showProgress = computed(() => this.isComfortable() && this.progress() > 0);

    public readonly overflowLabelLeft = computed(() => {
        const minWidth = isComfortableMode(this.cardMode())
            ? 2 * BAR_BORDER + 2 * BAR_PADDING_COMFORTABLE
            : 2 * BAR_BORDER + 2 * BAR_PADDING_COMPACT;
        return this.barLeft() + Math.max(this.barWidth(), minWidth) + OVERFLOW_LABEL_GAP;
    });

    public readonly overflowLabel = computed(() => {
        const padding = isComfortableMode(this.cardMode())
            ? BAR_PADDING_COMFORTABLE
            : BAR_PADDING_COMPACT;
        const contentWidth = this.barWidth() - 2 * BAR_BORDER - 2 * padding;
        const titleWidth = this.task().title.length * 7;
        return titleWidth > contentWidth ? this.task().title : null;
    });

    public onClick(): void {
        if (this._dragMoved) {
            this._dragMoved = false;
            return;
        }
        const issue = this.task();
        void this.router.navigate(['/project', issue.idProject, 'issue', issue.idIssuePublic]);
    }

    public onMouseEnter(): void {
        this.hovered.emit(this.task().idIssuePublic);
    }

    public onMouseLeave(): void {
        this.hovered.emit(null);
    }

    public onMouseDown(event: MouseEvent): void {
        if (event.button !== 0) return;
        event.preventDefault(); // prevent text selection during drag
        this._dragMoved = false;
        const onFirstMove = (): void => {
            this._dragMoved = true;
        };
        document.addEventListener('mousemove', onFirstMove, { once: true });
        this.dragStarted.emit({ taskId: this.task().idIssuePublic, event });
    }

    public onResizeMouseDown(event: MouseEvent): void {
        event.stopPropagation();
        event.preventDefault();
        this._dragMoved = true; // always suppress click after resize

        const task = this.task();
        const startX = event.clientX;
        const startWidth = this.barWidth();

        const onMouseMove = (moveEvent: MouseEvent): void => {
            const delta = moveEvent.clientX - startX;
            this._resizePreviewWidth.set(Math.max(MIN_BAR_WIDTH_PX, startWidth + delta));
        };

        const onMouseUp = (upEvent: MouseEvent): void => {
            const finalWidth = Math.max(MIN_BAR_WIDTH_PX, startWidth + (upEvent.clientX - startX));
            const endPixel = this.barLeft() + finalWidth;
            const endDate = this.timelineService.toDate(endPixel);
            const snappedEnd = this.timelineService.snapToNearest(endDate);
            const newEstimated = Math.max(
                3600,
                Math.round((snappedEnd.getTime() - task.scheduledAt.getTime()) / 1000)
            );

            // Hold the preview at the snapped width until fresh task data lands
            // (the constructor effect clears it) — no flash of the old width.
            const snappedEndDate = new Date(task.scheduledAt.getTime() + newEstimated * 1000);
            this._resizePreviewWidth.set(
                Math.max(
                    MIN_BAR_WIDTH_PX,
                    this.timelineService.toPixel(snappedEndDate) - this.barLeft()
                )
            );
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);

            this.resizeEnded.emit({ taskId: task.idIssuePublic, newEstimated });
        };

        // Listeners run inside zone so _resizePreviewWidth updates trigger CD on this OnPush component
        this.ngZone.run(() => {
            document.addEventListener('mousemove', onMouseMove);
            document.addEventListener('mouseup', onMouseUp);
        });
    }

    public onContextMenu(event: MouseEvent): void {
        event.preventDefault();
        this.contextMenu.emit({ taskId: this.task().idIssuePublic, event });
    }
}
