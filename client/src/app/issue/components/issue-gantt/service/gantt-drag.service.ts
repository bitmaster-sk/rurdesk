import { Injectable, NgZone, inject, signal, computed } from '@angular/core';
import { GanttTimelineService } from './gantt-timeline.service';
import { HandleSide } from '../constants/gantt-handle-side.enum';
import { format } from 'date-fns';

export enum DragMode {
    Idle = 'idle',
    Moving = 'moving',
    Resizing = 'resizing',
    DrawingRelation = 'drawingRelation',
    SchedulingBacklog = 'schedulingBacklog'
}

export interface DragState {
    mode: DragMode;
    taskId: number | null;
    startX: number;
    startDate: Date | null;
    currentDate: Date | null;
    startEstimated: number | null;
    scheduledAt: Date | null;
    lastClientX: number;
    lastClientY: number;
    sourceSide: HandleSide | null; // for drawingRelation
}

export interface RelationDropTarget {
    taskId: number;
    side: HandleSide;
}

@Injectable()
export class GanttDragService {
    private readonly timelineService = inject(GanttTimelineService);
    private readonly ngZone = inject(NgZone);

    public readonly state = signal<DragState>({
        mode: DragMode.Idle,
        taskId: null,
        startX: 0,
        startDate: null,
        currentDate: null,
        startEstimated: null,
        scheduledAt: null,
        lastClientX: 0,
        lastClientY: 0,
        sourceSide: null
    });

    public readonly isDragging = computed(() => this.state().mode !== DragMode.Idle);

    /** Emits the completed mode when mouseup fires. Host should watch this. */
    public readonly completed = signal<DragMode | null>(null);

    /** Stores the drop target detected on mouseup during drawingRelation. */
    private readonly _lastDropTarget = signal<RelationDropTarget | null>(null);
    public readonly lastDropTarget = this._lastDropTarget.asReadonly();

    /** Live drop target under the cursor while drawingRelation — drives the
     *  "release here to connect" highlight and the drawing-line snap. */
    private readonly _activeDropTarget = signal<RelationDropTarget | null>(null);
    public readonly activeDropTarget = this._activeDropTarget.asReadonly();

    public readonly tooltipText = computed(() => {
        const current = this.state().currentDate;
        if (!current) return '';
        return format(current, 'EEE d MMM, HH:mm');
    });

    /**
     * @param canvasOffsetX The X offset of the timeline canvas relative to viewport.
     *   Pass `container.getBoundingClientRect().left - container.scrollLeft` so that
     *   updatePosition can convert clientX to canvas-relative pixels.
     */
    private canvasOffsetX = 0;

    public setCanvasOffset(offsetX: number): void {
        this.canvasOffsetX = offsetX;
    }

    public startMove(taskId: number, startX: number, scheduledAt: Date): void {
        this.completed.set(null);
        this.state.set({
            mode: DragMode.Moving,
            taskId,
            startX,
            startDate: scheduledAt,
            currentDate: scheduledAt,
            startEstimated: null,
            scheduledAt,
            lastClientX: startX,
            lastClientY: 0,
            sourceSide: null
        });
        this.addGlobalListeners();
    }

    public startResize(taskId: number, startX: number, estimated: number, scheduledAt: Date): void {
        this.completed.set(null);
        this.state.set({
            mode: DragMode.Resizing,
            taskId,
            startX,
            startDate: null,
            currentDate: null,
            startEstimated: estimated,
            scheduledAt,
            lastClientX: startX,
            lastClientY: 0,
            sourceSide: null
        });
        this.addGlobalListeners();
    }

    public startRelationDraw(
        taskId: number,
        side: HandleSide,
        clientX: number,
        clientY: number
    ): void {
        this.completed.set(null);
        this._lastDropTarget.set(null);
        this.state.set({
            mode: DragMode.DrawingRelation,
            taskId,
            startX: 0,
            startDate: null,
            currentDate: null,
            startEstimated: null,
            scheduledAt: null,
            lastClientX: clientX,
            lastClientY: clientY,
            sourceSide: side
        });
        this.addGlobalListeners();
    }

    public startBacklogSchedule(taskId: number, startX: number): void {
        this.completed.set(null);
        this.state.set({
            mode: DragMode.SchedulingBacklog,
            taskId,
            startX,
            startDate: null,
            currentDate: null,
            startEstimated: null,
            scheduledAt: null,
            lastClientX: startX,
            lastClientY: 0,
            sourceSide: null
        });
        this.addGlobalListeners();
    }

    public updatePosition(clientX: number, clientY: number = 0): void {
        const current = this.state();
        if (current.mode === DragMode.Idle) return;

        if (current.mode === DragMode.Moving && current.startDate) {
            const deltaPixels = clientX - current.startX;
            const originDate = current.startDate;
            const pixelOfOrigin = this.timelineService.toPixel(originDate);
            const newDate = this.timelineService.toDate(pixelOfOrigin + deltaPixels);
            const snapped = this.timelineService.snapToNearest(newDate);
            this.state.update(s => ({
                ...s,
                currentDate: snapped,
                lastClientX: clientX
            }));
        }

        if (current.mode === DragMode.SchedulingBacklog) {
            const canvasPixel = clientX - this.canvasOffsetX;
            const date = this.timelineService.toDate(canvasPixel);
            const snapped = this.timelineService.snapToNearest(date);
            this.state.update(s => ({
                ...s,
                currentDate: snapped,
                lastClientX: clientX,
                lastClientY: clientY
            }));
        }

        if (current.mode === DragMode.Resizing) {
            this.state.update(s => ({ ...s, lastClientX: clientX }));
        }

        if (current.mode === DragMode.DrawingRelation) {
            this.state.update(s => ({ ...s, lastClientX: clientX, lastClientY: clientY }));
            const target = this.detectRelationDropTarget(clientX, clientY);
            const previous = this._activeDropTarget();
            if (target?.taskId !== previous?.taskId || target?.side !== previous?.side) {
                // Signal writes inside runOutsideAngular don't trigger CD on OnPush
                // consumers — re-enter the zone only when the target actually changes.
                this.ngZone.run(() => this._activeDropTarget.set(target));
            }
        }
    }

    public getMoveDelta(): { newScheduledAt: Date } | null {
        const current = this.state();
        if (current.mode !== DragMode.Moving || !current.currentDate) return null;
        return { newScheduledAt: current.currentDate };
    }

    public getBacklogScheduleResult(): {
        scheduledAt: Date;
        estimated: number;
    } | null {
        const current = this.state();
        if (current.mode !== DragMode.SchedulingBacklog || !current.currentDate) return null;
        return { scheduledAt: current.currentDate, estimated: 3600 };
    }

    public cancel(): void {
        this.state.set({
            mode: DragMode.Idle,
            taskId: null,
            startX: 0,
            startDate: null,
            currentDate: null,
            startEstimated: null,
            scheduledAt: null,
            lastClientX: 0,
            lastClientY: 0,
            sourceSide: null
        });
        this.completed.set(null);
        this._activeDropTarget.set(null);
        this.removeGlobalListeners();
    }

    public reset(): void {
        this.state.set({
            mode: DragMode.Idle,
            taskId: null,
            startX: 0,
            startDate: null,
            currentDate: null,
            startEstimated: null,
            scheduledAt: null,
            lastClientX: 0,
            lastClientY: 0,
            sourceSide: null
        });
        this.completed.set(null);
        this._lastDropTarget.set(null);
        this._activeDropTarget.set(null);
    }

    private onGlobalMouseMove = (event: MouseEvent): void => {
        this.updatePosition(event.clientX, event.clientY);
    };

    private onGlobalMouseUp = (event: MouseEvent): void => {
        const mode = this.state().mode;
        if (mode === DragMode.DrawingRelation) {
            this._lastDropTarget.set(this.detectRelationDropTarget(event.clientX, event.clientY));
            this._activeDropTarget.set(null);
        }
        this.removeGlobalListeners();
        // Signal the completed mode so the host can read state and act
        this.completed.set(mode);
    };

    /**
     * Finds the relation drop target under the cursor. A drop is accepted on a
     * connection handle or anywhere on a task bar (nearer edge picks the side) —
     * the 8px handle dot alone is too small a target. The source task never
     * matches, so the highlight can't suggest a self-relation.
     */
    private detectRelationDropTarget(clientX: number, clientY: number): RelationDropTarget | null {
        const el = document.elementFromPoint(clientX, clientY);
        if (!el) return null;

        const handleEl = el.closest<HTMLElement>('.connection-handle');
        if (handleEl) {
            const taskId = Number(handleEl.dataset['taskId']);
            const side = handleEl.dataset['side'] as HandleSide | undefined;
            if (taskId && side && taskId !== this.state().taskId) return { taskId, side };
            return null;
        }

        const barEl = el.closest<HTMLElement>('.gantt-bar');
        if (!barEl) return null;
        const taskId = Number(barEl.dataset['taskId']);
        if (!taskId || taskId === this.state().taskId) return null;
        const rect = barEl.getBoundingClientRect();
        const side = clientX < rect.left + rect.width / 2 ? HandleSide.Left : HandleSide.Right;
        return { taskId, side };
    }

    private onGlobalKeyDown = (event: KeyboardEvent): void => {
        if (event.key === 'Escape') {
            this.cancel();
        }
    };

    private addGlobalListeners(): void {
        this.ngZone.runOutsideAngular(() => {
            document.addEventListener('mousemove', this.onGlobalMouseMove);
            document.addEventListener('mouseup', this.onGlobalMouseUp);
            document.addEventListener('keydown', this.onGlobalKeyDown);
        });
    }

    private removeGlobalListeners(): void {
        document.removeEventListener('mousemove', this.onGlobalMouseMove);
        document.removeEventListener('mouseup', this.onGlobalMouseUp);
        document.removeEventListener('keydown', this.onGlobalKeyDown);
    }
}
