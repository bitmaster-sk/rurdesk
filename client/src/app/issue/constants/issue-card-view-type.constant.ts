export type IssueCardViewType =
    'CalendarComfort' | 'CalendarCompact' | 'GanttComfort' | 'GanttCompact';

export const CALENDAR_CARD_MODE_OPTIONS: { labelKey: string; value: IssueCardViewType }[] = [
    { labelKey: 'ISSUE.CARD_MODE.COMFORTABLE', value: 'CalendarComfort' },
    { labelKey: 'ISSUE.CARD_MODE.COMPACT', value: 'CalendarCompact' }
];

export const GANTT_CARD_MODE_OPTIONS: { labelKey: string; value: IssueCardViewType }[] = [
    { labelKey: 'ISSUE.CARD_MODE.COMFORTABLE', value: 'GanttComfort' },
    { labelKey: 'ISSUE.CARD_MODE.COMPACT', value: 'GanttCompact' }
];

export function isComfortableMode(type: IssueCardViewType): boolean {
    return type === 'CalendarComfort' || type === 'GanttComfort';
}
