import {
    startOfHour,
    addHours,
    differenceInHours,
    startOfDay,
    addDays,
    differenceInDays,
    startOfWeek,
    addWeeks,
    differenceInWeeks,
    startOfMonth,
    addMonths,
    differenceInMonths,
    getISOWeek,
    format
} from 'date-fns';
import { I18nService } from 'src/app/shared/i18n/i18n.service';

export enum GanttZoomLevel {
    Hour = 'Hour',
    QuarterDay = 'QuarterDay',
    Day = 'Day',
    Week = 'Week',
    Month = 'Month'
}

export interface ZoomConfig {
    columnWidthPx: number;
    showWeekend: boolean;
    /** Columns to add before the first task */
    paddingBefore: number;
    /** Columns to add after the last task end */
    paddingAfter: number;
    /** Minimum total columns for the timeline */
    minColumns: number;
    snapFn: (date: Date) => Date;
    advanceFn: (date: Date, count: number) => Date;
    diffFn: (start: Date, end: Date) => number;
    labelFn: (date: Date, i18n: I18nService) => string;
    headerRow1GroupFn: (date: Date) => string;
    headerRow1LabelFn: (date: Date) => string;
}

export const ZOOM_CONFIGS: Record<GanttZoomLevel, ZoomConfig> = {
    [GanttZoomLevel.Hour]: {
        columnWidthPx: 60,
        showWeekend: true,
        paddingBefore: 2,
        paddingAfter: 2,
        minColumns: 40,
        snapFn: date => startOfHour(date),
        advanceFn: (date, n) => addHours(date, n),
        diffFn: (start, end) => differenceInHours(end, start),
        labelFn: date => format(date, 'HH'),
        headerRow1GroupFn: date => format(date, 'yyyy-MM-dd'),
        headerRow1LabelFn: date => format(date, 'EEE d MMM')
    },
    [GanttZoomLevel.QuarterDay]: {
        columnWidthPx: 80,
        showWeekend: true,
        paddingBefore: 2,
        paddingAfter: 2,
        minColumns: 16,
        snapFn: date => {
            const d = new Date(date);
            d.setMinutes(0, 0, 0);
            d.setHours(Math.floor(d.getHours() / 6) * 6);
            return d;
        },
        advanceFn: (date, n) => addHours(date, n * 6),
        diffFn: (start, end) => Math.floor(differenceInHours(end, start) / 6),
        labelFn: (date, i18n) => {
            const period = Math.floor(date.getHours() / 6);
            if (period === 0) return i18n.instant('ISSUE.GANTT.QUARTER.NIGHT');
            if (period === 1) return i18n.instant('ISSUE.GANTT.QUARTER.MORNING');
            if (period === 2) return i18n.instant('ISSUE.GANTT.QUARTER.AFTERNOON');
            return i18n.instant('ISSUE.GANTT.QUARTER.EVENING');
        },
        headerRow1GroupFn: date => format(date, 'yyyy-MM-dd'),
        headerRow1LabelFn: date => format(date, 'EEE d MMM')
    },
    [GanttZoomLevel.Day]: {
        columnWidthPx: 40,
        showWeekend: true,
        paddingBefore: 2,
        paddingAfter: 2,
        minColumns: 40,
        snapFn: date => startOfDay(date),
        advanceFn: (date, n) => addDays(date, n),
        diffFn: (start, end) => differenceInDays(end, start),
        labelFn: date => format(date, 'd'),
        headerRow1GroupFn: date => format(date, 'yyyy-MM'),
        headerRow1LabelFn: date => format(date, 'MMMM yyyy')
    },
    [GanttZoomLevel.Week]: {
        columnWidthPx: 80,
        showWeekend: false,
        paddingBefore: 2,
        paddingAfter: 2,
        minColumns: 17,
        snapFn: date => startOfWeek(date, { weekStartsOn: 1 }),
        advanceFn: (date, n) => addWeeks(date, n),
        diffFn: (start, end) => differenceInWeeks(end, start),
        labelFn: date => `W${getISOWeek(date)}`,
        headerRow1GroupFn: date => format(date, 'yyyy-MM'),
        headerRow1LabelFn: date => format(date, 'MMMM yyyy')
    },
    [GanttZoomLevel.Month]: {
        columnWidthPx: 100,
        showWeekend: false,
        paddingBefore: 2,
        paddingAfter: 2,
        minColumns: 12,
        snapFn: date => startOfMonth(date),
        advanceFn: (date, n) => addMonths(date, n),
        diffFn: (start, end) => differenceInMonths(end, start),
        labelFn: date => format(date, 'MMM'),
        headerRow1GroupFn: date => format(date, 'yyyy'),
        headerRow1LabelFn: date => format(date, 'yyyy')
    }
};
