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
import { ExtendedIssue } from '../../../../model/extended-issue.model';
import { GanttRelation } from '../../model/gantt-relation.model';
import { IssueRelationType } from '../../../../constants/issue-relation-type.enum';
import { IssueRelationSubType } from '../../../../constants/issue-relation-subtype.enum';
import { HandleSide } from '../../constants/gantt-handle-side.enum';
import { MIN_BAR_WIDTH_PX, SMALL_BAR_THRESHOLD_PX } from '../gantt-task-bar/gantt-task-bar';
import { addSeconds } from 'date-fns';
import { ArrowDirection, routeArrow } from '../../service/gantt-arrow-routing';

interface ArrowPath {
    relationId: number;
    idProject: number;
    fromIdIssuePublic: number;
    path: string;
    isDirectional: boolean;
    isDrawIn: boolean;
    /** Stagger delay for the critical-path trace, null when not tracing. */
    traceDelayMs: number | null;
    relationTypeTranslationKey: string;
    relationSubTypeTranslationKey: string;
    lagLabel: string | null;
    lagLabelX: number;
    lagLabelY: number;
    midX: number;
    midY: number;
}

// Must match the CSS offset for .connection-handle--offset (right: -8px + translate(50%) = 8px past bar edge)
const MINIMAL_BAR_HANDLE_OFFSET = 8;

@Component({
    selector: 'app-gantt-arrow-layer',
    templateUrl: './gantt-arrow-layer.component.html',
    styleUrls: ['./gantt-arrow-layer.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class GanttArrowLayerComponent {
    private readonly timelineService = inject(GanttTimelineService);
    private readonly idHoveredRelation = signal<number | null>(null);
    public readonly idHoveredRelation$ = this.idHoveredRelation.asReadonly();

    public readonly tasks = input.required<ExtendedIssue[]>();
    public readonly relations = input.required<GanttRelation[]>();
    public readonly totalWidth = input.required<number>();
    public readonly totalHeight = input.required<number>();
    public readonly scrollLeft = input<number>(0);
    public readonly scrollTop = input<number>(0);
    public readonly viewportWidth = input<number>(0);
    public readonly viewportHeight = input<number>(0);
    public readonly selectedRelationId = input<number | null>(null);
    public readonly criticalRelationIds = input<Set<number>>(new Set());
    public readonly criticalRelationOrder = input<Map<number, number>>(new Map());
    public readonly isCriticalTracing = input<boolean>(false);
    public readonly isCriticalPathEnabled = input<boolean>(false);

    public readonly drawingLine = input<{
        sourceTaskId: number;
        sourceSide: HandleSide;
        clientX: number;
        clientY: number;
    } | null>(null);

    /** Live drop target of the in-progress draw — the line end snaps onto it. */
    public readonly dropTarget = input<{ taskId: number; side: HandleSide } | null>(null);

    /** Relation the user just drew — its arrow animates in (draw-along-path). */
    public readonly drawInRelation = input<{ from: number; to: number } | null>(null);

    /** True while bars glide to post-drop positions — arrows dim so their
     *  instant re-route doesn't clash with the still-moving bars. */
    public readonly isSettling = input<boolean>(false);

    private readonly svgEl = viewChild<ElementRef<SVGElement>>('svg');

    public readonly arrowClicked = output<number>();
    public readonly deleteRequested = output<{
        relationId: number;
        idProject: number;
        idIssuePublic: number;
    }>();

    private readonly VIEWPORT_BUFFER = 500;

    public readonly arrows = computed<ArrowPath[]>(() => {
        const tasks = this.tasks();
        const relations = this.relations();
        const rowHeight = this.timelineService.rowHeight();
        const idProject = tasks[0]?.idProject ?? 0;

        // Viewport bounds for culling (only render arrows with at least one endpoint visible)
        const vpLeft = this.scrollLeft() - this.VIEWPORT_BUFFER;
        const vpRight = this.scrollLeft() + this.viewportWidth() + this.VIEWPORT_BUFFER;
        const vpTop = this.scrollTop() - this.VIEWPORT_BUFFER;
        const vpBottom = this.scrollTop() + this.viewportHeight() + this.VIEWPORT_BUFFER;

        // Build task position lookup: idIssuePublic → { rowIndex, left, right, centerY }
        const taskPositions = new Map<
            number,
            {
                rowIndex: number;
                left: number;
                right: number;
                centerY: number;
            }
        >();

        tasks.forEach((task, index) => {
            if (!task.scheduledAt) return;
            const left = this.timelineService.toPixel(task.scheduledAt);
            const endDate = addSeconds(task.scheduledAt, task.estimated ?? 3600);
            const rawRight = this.timelineService.toPixel(endDate);
            // Persistent arrows always touch the rendered bar edge (bars never
            // shrink below MIN_BAR_WIDTH_PX) — the offset handle positions are
            // only relevant for the interactive drawing line (getHandlePoint).
            const right = Math.max(rawRight, left + MIN_BAR_WIDTH_PX);
            const centerY = index * rowHeight + rowHeight / 2;
            taskPositions.set(task.idIssuePublic, {
                rowIndex: index,
                left,
                right,
                centerY
            });
        });

        // First pass: resolve endpoints for visible relations
        interface Candidate {
            relation: GanttRelation;
            subType: IssueRelationSubType;
            sourceX: number;
            targetX: number;
            exitDirection: ArrowDirection;
            enterDirection: ArrowDirection;
            fromPos: { rowIndex: number; centerY: number };
            toPos: { rowIndex: number; centerY: number };
            exitLane: number;
            enterLane: number;
        }
        const candidates: Candidate[] = [];
        // While the critical-path trace plays, culling must not drop/recreate
        // arrows on scroll — a recreated arrow restarts its delayed animation.
        const isCullingActive = !this.isCriticalTracing();

        for (const relation of relations) {
            if (relation.direction !== IssueRelationDirection.Outbound) continue;
            const fromPos = taskPositions.get(relation.from.idIssuePublic);
            const toPos = taskPositions.get(relation.to.idIssuePublic);
            if (!fromPos || !toPos) continue;

            // Viewport culling: skip arrows where both endpoints are outside the visible area
            const fromVisible =
                fromPos.right >= vpLeft &&
                fromPos.left <= vpRight &&
                fromPos.centerY >= vpTop &&
                fromPos.centerY <= vpBottom;
            const toVisible =
                toPos.right >= vpLeft &&
                toPos.left <= vpRight &&
                toPos.centerY >= vpTop &&
                toPos.centerY <= vpBottom;
            if (isCullingActive && !fromVisible && !toVisible) continue;

            const subType = (relation.relationSubType ??
                IssueRelationSubType.FinishToStart) as IssueRelationSubType;
            const { sourceX, targetX, exitDirection, enterDirection } = this.getConnectionPoints(
                fromPos,
                toPos,
                subType
            );
            candidates.push({
                relation,
                subType,
                sourceX,
                targetX,
                exitDirection,
                enterDirection,
                fromPos,
                toPos,
                exitLane: 0,
                enterLane: 0
            });
        }

        // Second pass: lane assignment. Arrows sharing a task side fan out instead
        // of overlapping — nearer targets get the inner lanes so lines nest.
        const byExit = new Map<string, Candidate[]>();
        const byEnter = new Map<string, Candidate[]>();
        for (const c of candidates) {
            const exitKey = `${c.relation.from.idIssuePublic}:${c.exitDirection}`;
            const enterKey = `${c.relation.to.idIssuePublic}:${c.enterDirection}`;
            (byExit.get(exitKey) ?? byExit.set(exitKey, []).get(exitKey)!).push(c);
            (byEnter.get(enterKey) ?? byEnter.set(enterKey, []).get(enterKey)!).push(c);
        }
        for (const group of byExit.values()) {
            group
                .sort(
                    (a, b) =>
                        Math.abs(a.toPos.rowIndex - a.fromPos.rowIndex) -
                        Math.abs(b.toPos.rowIndex - b.fromPos.rowIndex)
                )
                .forEach((c, i) => (c.exitLane = i));
        }
        for (const group of byEnter.values()) {
            group
                .sort(
                    (a, b) =>
                        Math.abs(a.toPos.rowIndex - a.fromPos.rowIndex) -
                        Math.abs(b.toPos.rowIndex - b.fromPos.rowIndex)
                )
                .forEach((c, i) => (c.enterLane = i));
        }

        const arrows: ArrowPath[] = [];
        const drawIn = this.drawInRelation();
        const isTracing = this.isCriticalTracing();
        const traceOrder = this.criticalRelationOrder();

        for (const c of candidates) {
            const { relation, subType } = c;
            const isDirectional =
                subType === IssueRelationSubType.FinishToStart ||
                subType === IssueRelationSubType.StartToFinish;
            const isDrawIn =
                drawIn !== null &&
                relation.from.idIssuePublic === drawIn.from &&
                relation.to.idIssuePublic === drawIn.to;
            const traceIndex = isTracing ? traceOrder.get(relation.idIssueRelation) : undefined;
            const traceDelayMs = traceIndex !== undefined ? traceIndex * 120 : null;

            const { path, midX, midY } = routeArrow({
                sourceX: c.sourceX,
                sourceY: c.fromPos.centerY,
                targetX: c.targetX,
                targetY: c.toPos.centerY,
                exitDirection: c.exitDirection,
                enterDirection: c.enterDirection,
                sourceRowIndex: c.fromPos.rowIndex,
                targetRowIndex: c.toPos.rowIndex,
                rowHeight,
                exitLane: c.exitLane,
                enterLane: c.enterLane
            });

            let lagLabel: string | null = null;
            if (relation.lagMinutes) {
                const totalMinutes = relation.lagMinutes;
                if (totalMinutes >= 60) {
                    lagLabel = `${Math.floor(totalMinutes / 60)}h`;
                    if (totalMinutes % 60 > 0) lagLabel += `${totalMinutes % 60}m`;
                } else {
                    lagLabel = `${totalMinutes}m`;
                }
            }

            arrows.push({
                relationId: relation.idIssueRelation,
                idProject,
                fromIdIssuePublic: relation.from.idIssuePublic,
                path,
                isDirectional,
                isDrawIn,
                traceDelayMs,
                relationTypeTranslationKey: this.getRelationTypeTranslationKey(
                    relation.relationType
                ),
                relationSubTypeTranslationKey: this.getRelationSubTypeTranslationKey(
                    relation.relationSubType
                ),
                lagLabel,
                lagLabelX: midX,
                lagLabelY: midY - 8,
                midX,
                midY
            });
        }

        return arrows;
    });

    // SVG has no z-index — paint (and hit-test) order is document order. Keep the
    // hovered arrow last so it renders above overlapping siblings, with the selected
    // one just below it.
    public readonly displayArrows = computed<ArrowPath[]>(() => {
        const hoveredId = this.idHoveredRelation$();
        const selectedId = this.selectedRelationId();
        if (hoveredId === null && selectedId === null) return this.arrows();
        const rank = (a: ArrowPath): number =>
            a.relationId === hoveredId ? 2 : a.relationId === selectedId ? 1 : 0;
        return [...this.arrows()].sort((a, b) => rank(a) - rank(b));
    });

    private getConnectionPoints(
        fromPos: { left: number; right: number },
        toPos: { left: number; right: number },
        subType: IssueRelationSubType
    ): {
        sourceX: number;
        targetX: number;
        exitDirection: ArrowDirection;
        enterDirection: ArrowDirection;
    } {
        switch (subType) {
            case IssueRelationSubType.FinishToStart:
                return {
                    sourceX: fromPos.right,
                    targetX: toPos.left,
                    exitDirection: 'right',
                    enterDirection: 'left'
                };
            case IssueRelationSubType.StartToStart:
                return {
                    sourceX: fromPos.left,
                    targetX: toPos.left,
                    exitDirection: 'left',
                    enterDirection: 'left'
                };
            case IssueRelationSubType.FinishToFinish:
                return {
                    sourceX: fromPos.right,
                    targetX: toPos.right,
                    exitDirection: 'right',
                    enterDirection: 'right'
                };
            case IssueRelationSubType.StartToFinish:
                return {
                    sourceX: fromPos.left,
                    targetX: toPos.right,
                    exitDirection: 'left',
                    enterDirection: 'right'
                };
            default:
                return {
                    sourceX: fromPos.right,
                    targetX: toPos.left,
                    exitDirection: 'right',
                    enterDirection: 'left'
                };
        }
    }

    public readonly drawingArrowPath = computed(() => {
        const line = this.drawingLine();
        if (!line) return null;

        const svgNative = this.svgEl()?.nativeElement;
        if (!svgNative) return null;

        const source = this.getHandlePoint(line.sourceTaskId, line.sourceSide);
        if (!source) return null;

        // Snap the free end onto the active drop target's handle when there is one
        const target = this.dropTarget();
        const snapped = target ? this.getHandlePoint(target.taskId, target.side) : null;

        let targetX: number;
        let targetY: number;
        if (snapped) {
            targetX = snapped.x;
            targetY = snapped.y;
        } else {
            const svgRect = svgNative.getBoundingClientRect();
            targetX = line.clientX - svgRect.left;
            targetY = line.clientY - svgRect.top;
        }

        return `M ${source.x} ${source.y} L ${targetX} ${targetY}`;
    });

    /**
     * Canvas coordinates of a task's connection handle, or null if unscheduled.
     * Mirrors the handle CSS: small bars (< SMALL_BAR_THRESHOLD_PX) render their
     * handles MINIMAL_BAR_HANDLE_OFFSET px outside each edge, so the drawing
     * line snaps onto the visible dot.
     */
    private getHandlePoint(taskId: number, side: HandleSide): { x: number; y: number } | null {
        const tasks = this.tasks();
        const task = tasks.find(t => t.idIssuePublic === taskId);
        if (!task?.scheduledAt) return null;

        const rowHeight = this.timelineService.rowHeight();
        const rowIndex = tasks.indexOf(task);
        const centerY = rowIndex * rowHeight + rowHeight / 2;
        const left = this.timelineService.toPixel(task.scheduledAt);
        const endDate = addSeconds(task.scheduledAt, task.estimated ?? 3600);
        const rawRight = this.timelineService.toPixel(endDate);
        const right = Math.max(rawRight, left + MIN_BAR_WIDTH_PX);
        const isSmallBar = right - left < SMALL_BAR_THRESHOLD_PX;
        const handleLeft = isSmallBar ? left - MINIMAL_BAR_HANDLE_OFFSET : left;
        const handleRight = isSmallBar ? right + MINIMAL_BAR_HANDLE_OFFSET : right;
        return { x: side === HandleSide.Left ? handleLeft : handleRight, y: centerY };
    }

    public onArrowClick(relationId: number): void {
        this.arrowClicked.emit(relationId);
    }

    public onArrowMouseEnter(relationId: number): void {
        this.idHoveredRelation.set(relationId);
    }

    public onArrowMouseLeave(): void {
        this.idHoveredRelation.set(null);
    }

    private getRelationTypeTranslationKey(relationType: IssueRelationType): string {
        switch (relationType) {
            case IssueRelationType.Hierarchy:
                return 'RELATION.HIERARCHY';
            case IssueRelationType.Schedule:
                return 'RELATION.SCHEDULE';
            case IssueRelationType.Duplicates:
                return 'RELATION.DUPLICATES';
            case IssueRelationType.RelatesTo:
                return 'RELATION.RELATES_TO';
            default:
                return 'RELATION.BASIC';
        }
    }

    private getRelationSubTypeTranslationKey(relationSubType: IssueRelationSubType | null): string {
        switch (relationSubType) {
            case IssueRelationSubType.FinishToStart:
                return 'RELATION.FINISH_TO_START';
            case IssueRelationSubType.StartToStart:
                return 'RELATION.START_TO_START';
            case IssueRelationSubType.FinishToFinish:
                return 'RELATION.FINISH_TO_FINISH';
            case IssueRelationSubType.StartToFinish:
                return 'RELATION.START_TO_FINISH';
            case IssueRelationSubType.Parent:
                return 'RELATION.PARENT';
            case IssueRelationSubType.Child:
                return 'RELATION.CHILD';
            default:
                return 'RELATION.BASIC';
        }
    }
}
