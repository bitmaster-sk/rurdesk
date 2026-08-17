import { Injectable, computed, inject, signal } from '@angular/core';
import { differenceInMilliseconds, isSameDay, isWeekend as dateFnsIsWeekend } from 'date-fns';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { GanttZoomLevel, ZOOM_CONFIGS } from '../constants/gantt-zoom-config';
import { STORAGE_KEY_ZOOM } from '../constants/gantt-storage-keys';

export interface GanttColumn {
    date: Date;
    left: number;
    width: number;
    label: string;
    isToday: boolean;
    isWeekend: boolean;
}

export interface GanttHeaderCell {
    label: string;
    left: number;
    width: number;
    isWeekend?: boolean;
}

export interface GanttHeaderRow {
    cells: GanttHeaderCell[];
}

@Injectable()
export class GanttTimelineService {
    private readonly i18n = inject(I18nService);

    public readonly zoomLevel = signal<GanttZoomLevel>(this.loadZoomLevel());
    public readonly rowHeight = signal<number>(64);

    public readonly config = computed(() => ZOOM_CONFIGS[this.zoomLevel()]);
    public readonly columnWidth = computed(() => this.config().columnWidthPx);

    private readonly rangeStart = signal<Date>(new Date());
    private readonly rangeEnd = signal<Date>(new Date());

    public setRange(start: Date, end: Date): void {
        this.rangeStart.set(start);
        this.rangeEnd.set(end);
    }

    public setZoom(level: GanttZoomLevel): void {
        this.zoomLevel.set(level);
        localStorage.setItem(STORAGE_KEY_ZOOM, level);
    }

    public toPixel(date: Date): number {
        const cfg = this.config();
        const origin = cfg.snapFn(this.rangeStart());
        const millisPerColumn = differenceInMilliseconds(cfg.advanceFn(origin, 1), origin);
        const elapsed = differenceInMilliseconds(date, origin);
        return (elapsed / millisPerColumn) * cfg.columnWidthPx;
        // NOTE: For Month zoom, months have variable lengths. This linear mapping
        // causes slight misalignment between bar positions and column boundaries
        // in months that differ in length from the first month. Acceptable for
        // overview zoom; revisit with column-aware mapping if precision is needed.
    }

    public toDate(pixel: number): Date {
        const cfg = this.config();
        const origin = cfg.snapFn(this.rangeStart());
        const millisPerColumn = differenceInMilliseconds(cfg.advanceFn(origin, 1), origin);
        const elapsed = (pixel / cfg.columnWidthPx) * millisPerColumn;
        return new Date(origin.getTime() + elapsed);
    }

    public snapToGrid(date: Date): Date {
        return this.config().snapFn(date);
    }

    /**
     * Snaps to the nearest column boundary (not just the previous one like
     * snapToGrid). Past the midpoint of a column the date rounds up — used by
     * drag/resize so the user doesn't have to hit the very end of a column.
     */
    public snapToNearest(date: Date): Date {
        const cfg = this.config();
        const floor = cfg.snapFn(date);
        const ceil = cfg.advanceFn(floor, 1);
        const midpoint = floor.getTime() + (ceil.getTime() - floor.getTime()) / 2;
        return date.getTime() >= midpoint ? ceil : floor;
    }

    // Column computation as a computed signal — Angular's reactive graph handles caching.
    // Recomputes automatically when zoomLevel, rangeStart, or rangeEnd change.
    public readonly columns = computed<GanttColumn[]>(() => {
        const cfg = this.config();
        const start = cfg.snapFn(this.rangeStart());
        const end = this.rangeEnd();
        const result: GanttColumn[] = [];
        const today = new Date();

        let current = start;
        let index = 0;
        while (current <= end) {
            result.push({
                date: current,
                left: index * cfg.columnWidthPx,
                width: cfg.columnWidthPx,
                label: cfg.labelFn(current, this.i18n),
                isToday: isSameDay(current, today),
                isWeekend: cfg.showWeekend && dateFnsIsWeekend(current)
            });
            current = cfg.advanceFn(current, 1);
            index++;
        }
        return result;
    });

    // Keep getColumns() for callers that use the imperative API.
    public getColumns(): GanttColumn[] {
        return this.columns();
    }

    public getHeaderRows(): GanttHeaderRow[] {
        const columns = this.getColumns();
        const cfg = this.config();

        // Row 1: grouped headers (month, date, year depending on zoom)
        const groups = new Map<string, { label: string; left: number; width: number }>();
        for (const col of columns) {
            const groupKey = cfg.headerRow1GroupFn(col.date);
            const existing = groups.get(groupKey);
            if (existing) {
                existing.width += col.width;
            } else {
                groups.set(groupKey, {
                    label: cfg.headerRow1LabelFn(col.date),
                    left: col.left,
                    width: col.width
                });
            }
        }

        // Row 2: individual column headers
        const row2: GanttHeaderCell[] = columns.map(col => ({
            label: col.label,
            left: col.left,
            width: col.width,
            isWeekend: cfg.showWeekend && col.isWeekend
        }));

        return [{ cells: Array.from(groups.values()) }, { cells: row2 }];
    }

    public getTodayPixel(): number {
        return this.toPixel(new Date());
    }

    public getTotalWidth(): number {
        const columns = this.getColumns();
        if (columns.length === 0) return 0;
        const last = columns[columns.length - 1];
        return last.left + last.width;
    }

    /**
     * Computes the timeline start/end from task dates, applying per-zoom padding and
     * enforcing the minimum column count. Safe to call with an empty task list —
     * falls back to today as the content anchor.
     */
    public computeRange(tasks: { scheduledAt?: Date | null; estimated?: number | null }[]): {
        start: Date;
        end: Date;
    } {
        const cfg = this.config();

        const scheduled = tasks.filter(
            (t): t is { scheduledAt: Date; estimated?: number | null } => t.scheduledAt != null
        );

        let contentStart: Date;
        let contentEnd: Date;

        if (scheduled.length === 0) {
            const now = new Date();
            contentStart = now;
            contentEnd = now;
        } else {
            contentStart = scheduled.reduce(
                (min, t) => (t.scheduledAt < min ? t.scheduledAt : min),
                scheduled[0].scheduledAt
            );
            contentEnd = scheduled.reduce(
                (max, t) => {
                    const taskEnd = new Date(
                        t.scheduledAt.getTime() + (t.estimated ?? 3600) * 1000
                    );
                    return taskEnd > max ? taskEnd : max;
                },
                new Date(
                    scheduled[0].scheduledAt.getTime() + (scheduled[0].estimated ?? 3600) * 1000
                )
            );
        }

        // Apply padding — advanceFn supports negative counts via date-fns
        let start = cfg.snapFn(cfg.advanceFn(contentStart, -cfg.paddingBefore));
        let end = cfg.advanceFn(contentEnd, cfg.paddingAfter);

        // Enforce minimum column count, extending the end if needed
        const columnCount = cfg.diffFn(start, end);
        if (columnCount < cfg.minColumns) {
            end = cfg.advanceFn(start, cfg.minColumns);
        }

        return { start, end };
    }

    private loadZoomLevel(): GanttZoomLevel {
        const stored = localStorage.getItem(STORAGE_KEY_ZOOM);
        const valid = Object.values(GanttZoomLevel);
        return valid.includes(stored as GanttZoomLevel)
            ? (stored as GanttZoomLevel)
            : GanttZoomLevel.Week;
    }
}
