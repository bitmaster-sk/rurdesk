import {
    ChangeDetectionStrategy,
    Component,
    ComponentRef,
    ElementRef,
    NgZone,
    OnDestroy,
    computed,
    effect,
    inject,
    input,
    output,
    signal,
    viewChild,
    AfterViewInit
} from '@angular/core';
import { Router } from '@angular/router';
import {
    ConnectedPosition,
    FlexibleConnectedPositionStrategy,
    Overlay,
    OverlayRef
} from '@angular/cdk/overlay';
import { ComponentPortal } from '@angular/cdk/portal';
import { UiTooltipComponent } from '../../../../../ui/components/tooltip/tooltip.component';
import {
    GanttTimelineService,
    GanttColumn,
    GanttHeaderRow
} from '../../service/gantt-timeline.service';
import { ScheduledIssue } from '../../../../model/extended-issue.model';
import { IssueCardViewType } from '../../../../constants/issue-card-view-type.constant';
import { GanttRelation } from '../../model/gantt-relation.model';
import { HandleSide } from '../../constants/gantt-handle-side.enum';
import { RelationDropTarget } from '../../service/gantt-drag.service';

const DRAG_TOOLTIP_POSITIONS: ConnectedPosition[] = [
    { originX: 'center', originY: 'top', overlayX: 'center', overlayY: 'bottom', offsetY: -8 },
    { originX: 'center', originY: 'bottom', overlayX: 'center', overlayY: 'top', offsetY: 8 }
];

@Component({
    selector: 'app-gantt-timeline-body',
    templateUrl: './gantt-timeline-body.html',
    styleUrl: './gantt-timeline-body.scss',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class GanttTimelineBodyComponent implements AfterViewInit, OnDestroy {
    private readonly router = inject(Router);
    private readonly ngZone = inject(NgZone);
    private readonly overlay = inject(Overlay);
    public readonly timelineService = inject(GanttTimelineService);

    private readonly scrollContainer =
        viewChild.required<ElementRef<HTMLDivElement>>('scrollContainer');
    private readonly canvas = viewChild.required<ElementRef<HTMLDivElement>>('canvas');

    public readonly tasks = input.required<ScheduledIssue[]>();
    public readonly columns = input.required<GanttColumn[]>();
    public readonly headerRows = input.required<GanttHeaderRow[]>();
    public readonly cardMode = input.required<IssueCardViewType>();
    public readonly criticalTaskIds = input<Set<number>>(new Set());
    public readonly criticalRelationIds = input<Set<number>>(new Set());
    public readonly criticalRelationOrder = input<Map<number, number>>(new Map());
    public readonly isCriticalTracing = input<boolean>(false);
    public readonly isCriticalPathEnabled = input<boolean>(false);
    public readonly ghostBars = input<
        { taskId: number; left: number; top: number; width: number; tooltipText: string | null }[]
    >([]);
    public readonly relations = input<GanttRelation[]>([]);
    public readonly selectedTaskIndex = input<number | null>(null);
    public readonly selectedRelationId = input<number | null>(null);
    public readonly drawingLine = input<{
        sourceTaskId: number;
        sourceSide: HandleSide;
        clientX: number;
        clientY: number;
    } | null>(null);
    public readonly relationDropTarget = input<RelationDropTarget | null>(null);
    public readonly drawInRelation = input<{ from: number; to: number } | null>(null);
    /** taskId → stagger delay (ms) for the post-drop domino settle. */
    public readonly cascadeSlide = input<Map<number, number>>(new Map());

    public readonly scrolled = output<{
        scrollTop: number;
        scrollLeft: number;
    }>();
    public readonly taskHovered = output<number | null>();
    public readonly barDragStarted = output<{
        taskId: number;
        event: MouseEvent;
    }>();
    public readonly barResizeEnded = output<{
        taskId: number;
        newEstimated: number;
    }>();
    public readonly barContextMenu = output<{
        taskId: number;
        event: MouseEvent;
    }>();
    public readonly connectionDragStarted = output<{
        taskId: number;
        side: HandleSide;
        event: MouseEvent;
    }>();
    public readonly arrowClicked = output<number>();
    public readonly arrowDeleteRequested = output<{
        relationId: number;
        idProject: number;
        idIssuePublic: number;
    }>();

    // Scroll state for arrow layer viewport culling
    public readonly scrollLeft = signal<number>(0);
    public readonly scrollTop = signal<number>(0);
    public readonly viewportWidth = signal<number>(0);
    public readonly viewportHeight = signal<number>(0);

    public readonly totalWidth = computed(() => this.timelineService.getTotalWidth());
    public readonly totalHeight = computed(
        () => this.tasks().length * this.timelineService.rowHeight()
    );
    public readonly todayLeft = computed(() => this.timelineService.getTodayPixel());

    private isSyncingScroll = false;

    private dragTooltipOverlayRef: OverlayRef | null = null;
    private dragTooltipRef: ComponentRef<UiTooltipComponent> | null = null;
    private dragTooltipPosition: FlexibleConnectedPositionStrategy | null = null;

    public constructor() {
        this.effectDragTooltip();
    }

    private effectDragTooltip(): void {
        effect(() => {
            const ghost = this.ghostBars().find(bar => bar.tooltipText);
            this.scrollLeft();
            this.scrollTop();
            if (!ghost?.tooltipText) {
                this.hideDragTooltip();
                return;
            }
            this.showDragTooltip(ghost.tooltipText, ghost.left, ghost.top, ghost.width);
        });
    }

    public ngOnDestroy(): void {
        this.hideDragTooltip();
    }

    public ngAfterViewInit(): void {
        // Scroll to today on init
        const todayPx = this.todayLeft();
        const container = this.scrollContainer().nativeElement;
        container.scrollLeft = Math.max(0, todayPx - container.clientWidth / 3);
        this.viewportWidth.set(container.clientWidth);
        this.viewportHeight.set(container.clientHeight);
    }

    public onScroll(): void {
        if (this.isSyncingScroll) {
            this.isSyncingScroll = false;
            return;
        }
        const el = this.scrollContainer().nativeElement;
        this.scrollLeft.set(el.scrollLeft);
        this.scrollTop.set(el.scrollTop);
        this.viewportWidth.set(el.clientWidth);
        this.viewportHeight.set(el.clientHeight);
        this.scrolled.emit({
            scrollTop: el.scrollTop,
            scrollLeft: el.scrollLeft
        });
    }

    public syncScrollFrom(scrollTop: number): void {
        this.isSyncingScroll = true;
        this.scrollContainer().nativeElement.scrollTop = scrollTop;
    }

    public scrollToPixel(px: number): void {
        const container = this.scrollContainer().nativeElement;
        container.scrollLeft = Math.max(0, px - container.clientWidth / 3);
    }

    /** Eased scroll to the today marker, with a "you are here" pulse on arrival. */
    public scrollToToday(): void {
        const container = this.scrollContainer().nativeElement;
        container.scrollTo({
            left: Math.max(0, this.todayLeft() - container.clientWidth / 3),
            behavior: 'smooth'
        });
        this.isTodayPulsing.set(false);
        if (this.todayPulseTimer) clearTimeout(this.todayPulseTimer);
        // Re-add the class next frame so the animation replays on repeated presses
        requestAnimationFrame(() => this.isTodayPulsing.set(true));
        this.todayPulseTimer = setTimeout(() => this.isTodayPulsing.set(false), 1400);
    }

    public readonly isTodayPulsing = signal(false);
    private todayPulseTimer: ReturnType<typeof setTimeout> | null = null;

    public panHorizontal(deltaPixels: number): void {
        const container = this.scrollContainer().nativeElement;
        container.scrollLeft = Math.max(0, container.scrollLeft + deltaPixels);
    }

    public getScrollContainer(): HTMLDivElement {
        return this.scrollContainer().nativeElement;
    }

    private showDragTooltip(text: string, left: number, top: number, width: number): void {
        const canvasRect = this.canvas().nativeElement.getBoundingClientRect();
        const origin = {
            x: canvasRect.left + left,
            y: canvasRect.top + top,
            width,
            height: this.timelineService.rowHeight() - 4
        };

        if (!this.dragTooltipOverlayRef) {
            this.dragTooltipPosition = this.overlay
                .position()
                .flexibleConnectedTo(origin)
                .withPositions(DRAG_TOOLTIP_POSITIONS)
                .withPush(true);
            this.dragTooltipOverlayRef = this.overlay.create({
                positionStrategy: this.dragTooltipPosition,
                scrollStrategy: this.overlay.scrollStrategies.noop(),
                hasBackdrop: false
            });
            this.dragTooltipRef = this.dragTooltipOverlayRef.attach(
                new ComponentPortal(UiTooltipComponent)
            );
        } else {
            this.dragTooltipPosition?.setOrigin(origin);
        }

        this.dragTooltipRef?.setInput('text', text);
        this.dragTooltipRef?.changeDetectorRef.detectChanges();
        this.dragTooltipOverlayRef.updatePosition();
    }

    private hideDragTooltip(): void {
        if (!this.dragTooltipOverlayRef) return;
        this.dragTooltipOverlayRef.dispose();
        this.dragTooltipOverlayRef = null;
        this.dragTooltipRef = null;
        this.dragTooltipPosition = null;
    }

    public onDoubleClick(event: MouseEvent): void {
        const container = this.scrollContainer().nativeElement;
        const rect = container.getBoundingClientRect();
        const pixelX = event.clientX - rect.left + container.scrollLeft;
        const clickedDate = this.timelineService.toDate(pixelX);
        const snapped = this.timelineService.snapToGrid(clickedDate);

        // Navigate to create issue with scheduledAt
        // idProject extracted from current route by parent
        void this.router.navigate([], {
            queryParams: { scheduledAt: snapped.toISOString() },
            queryParamsHandling: 'merge'
        });
    }

    public onMiddleMouseDown(event: MouseEvent): void {
        if (event.button !== 1) return;
        event.preventDefault(); // suppress scroll-lock cursor

        const startX = event.clientX;
        const container = this.scrollContainer().nativeElement;
        const startScrollLeft = container.scrollLeft;

        const onMouseMove = (moveEvent: MouseEvent): void => {
            // Inverted: dragging right pans left (natural pan feel)
            const deltaX = startX - moveEvent.clientX;
            container.scrollLeft = Math.max(0, startScrollLeft + deltaX);
        };

        const onMouseUp = (upEvent: MouseEvent): void => {
            if (upEvent.button === 1) {
                document.removeEventListener('mousemove', onMouseMove);
                document.removeEventListener('mouseup', onMouseUp);
            }
        };

        this.ngZone.runOutsideAngular(() => {
            document.addEventListener('mousemove', onMouseMove);
            document.addEventListener('mouseup', onMouseUp);
        });
    }
}
