import { isSameDay, subDays } from 'date-fns';

export enum UiDaySelectFormat {
    Short = 'short',
    Long = 'long'
}

export enum UiDaySelectRelative {
    Today = 'today',
    Yesterday = 'yesterday'
}

export abstract class UiDaySelectLabel {
    public static relative(value: Date, today: Date): UiDaySelectRelative | null {
        if (isSameDay(value, today)) {
            return UiDaySelectRelative.Today;
        }
        if (isSameDay(value, subDays(today, 1))) {
            return UiDaySelectRelative.Yesterday;
        }
        return null;
    }
}
