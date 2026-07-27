import { GanttZoomLevel } from './gantt-zoom-config';

export const ZOOM_LEVELS: GanttZoomLevel[] = Object.values(GanttZoomLevel);

export const ZOOM_OPTIONS: { labelKey: string; value: GanttZoomLevel }[] = [
    { labelKey: 'ISSUE.GANTT.ZOOM.HOUR', value: GanttZoomLevel.Hour },
    {
        labelKey: 'ISSUE.GANTT.ZOOM.QUARTER_DAY',
        value: GanttZoomLevel.QuarterDay
    },
    { labelKey: 'ISSUE.GANTT.ZOOM.DAY', value: GanttZoomLevel.Day },
    { labelKey: 'ISSUE.GANTT.ZOOM.WEEK', value: GanttZoomLevel.Week },
    { labelKey: 'ISSUE.GANTT.ZOOM.MONTH', value: GanttZoomLevel.Month }
];
