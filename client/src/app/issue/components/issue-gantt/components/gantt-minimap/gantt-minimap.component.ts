import {
    ChangeDetectionStrategy,
    Component,
    ElementRef,
    AfterViewInit,
    NgZone,
    OnDestroy,
    computed,
    inject,
    input,
    output,
    signal,
    viewChild
} from '@angular/core';
import { GanttTimelineService } from '../../service/gantt-timeline.service';
import { ExtendedIssue } from '../../../../model/extended-issue.model';
import { addSeconds } from 'date-fns';

const MINIMAP_HEIGHT = 40;

@Component({
    selector: 'app-gantt-minimap',
    templateUrl: './gantt-minimap.component.html',
    styleUrls: ['./gantt-minimap.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class GanttMinimapComponent implements AfterViewInit, OnDestroy {
    private readonly timelineService = inject(GanttTimelineService);
    private readonly ngZone = inject(NgZone);
    private readonly minimap = viewChild.required<ElementRef<HTMLDivElement>>('minimap');

    public readonly tasks = input.required<ExtendedIssue[]>();
    public readonly viewportScrollLeft = input<number>(0);
    public readonly viewportWidth = input<number>(0);

    public readonly navigated = output<number>();

    // Minimap width tracked via ResizeObserver for reactivity
    public readonly minimapWidth = signal<number>(400);
    private resizeObserver: ResizeObserver | null = null;

    public ngAfterViewInit(): void {
        const el = this.minimap().nativeElement;
        this.minimapWidth.set(el.clientWidth);
        this.resizeObserver = new ResizeObserver(entries => {
            for (const entry of entries) {
                this.minimapWidth.set(entry.contentRect.width);
            }
        });
        this.resizeObserver.observe(el);
    }

    public ngOnDestroy(): void {
        this.resizeObserver?.disconnect();
    }

    public readonly totalTimelineWidth = computed(() => this.timelineService.getTotalWidth());

    public readonly scale = computed(() => {
        const total = this.totalTimelineWidth();
        return total > 0 ? this.minimapWidth() / total : 1;
    });

    // Viewport indicator position
    public readonly viewportIndicatorLeft = computed(
        () => this.viewportScrollLeft() * this.scale()
    );

    public readonly viewportIndicatorWidth = computed(() =>
        Math.max(20, this.viewportWidth() * this.scale())
    );

    // Task bars (compressed)
    public readonly minimapBars = computed(() => {
        const tasks = this.tasks();
        const scale = this.scale();
        const rowCount = tasks.length;
        const barHeight = Math.max(1, MINIMAP_HEIGHT / Math.max(rowCount, 1));

        return tasks
            .map((task, index) => {
                if (!task.scheduledAt) return null;

                const left = this.timelineService.toPixel(task.scheduledAt) * scale;
                const endDate = addSeconds(task.scheduledAt, task.estimated ?? 3600);
                const right = this.timelineService.toPixel(endDate) * scale;
                const width = Math.max(2, right - left);

                return {
                    left,
                    top: index * barHeight,
                    width,
                    height: Math.max(1, barHeight - 1),
                    color: task.severity?.color ?? 'var(--ui-color-primary)'
                };
            })
            .filter(Boolean);
    });

    private isDraggingViewport = false;
    private dragStartX = 0;
    private dragStartScrollLeft = 0;

    public onMinimapClick(event: MouseEvent): void {
        const rect = this.minimap().nativeElement.getBoundingClientRect();
        const clickX = event.clientX - rect.left;
        const scrollLeft = clickX / this.scale() - this.viewportWidth() / 2;
        this.navigated.emit(Math.max(0, scrollLeft));
    }

    public onViewportDragStart(event: MouseEvent): void {
        event.stopPropagation();
        event.preventDefault();
        this.isDraggingViewport = true;
        this.dragStartX = event.clientX;
        this.dragStartScrollLeft = this.viewportScrollLeft();

        const onMouseMove = (moveEvent: MouseEvent): void => {
            if (!this.isDraggingViewport) return;
            const deltaX = moveEvent.clientX - this.dragStartX;
            const scrollDelta = deltaX / this.scale();
            this.navigated.emit(Math.max(0, this.dragStartScrollLeft + scrollDelta));
        };

        const onMouseUp = (): void => {
            this.isDraggingViewport = false;
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);
        };

        this.ngZone.runOutsideAngular(() => {
            document.addEventListener('mousemove', onMouseMove);
            document.addEventListener('mouseup', onMouseUp);
        });
    }
}
