import type { FormControl } from '@angular/forms';
import { addDays, startOfToday, subDays } from 'date-fns';
import { TrackDayValidator } from './track-day.validator';

function control(value: Date | null): FormControl<Date | null> {
    return { value } as unknown as FormControl<Date | null>;
}

describe('TrackDayValidator.notInFuture', () => {
    it('accepts today, including later in the day', () => {
        expect(TrackDayValidator.notInFuture(control(startOfToday()))).toBeNull();
        expect(TrackDayValidator.notInFuture(control(new Date()))).toBeNull();
    });

    it('accepts a past day', () => {
        expect(TrackDayValidator.notInFuture(control(subDays(startOfToday(), 1)))).toBeNull();
    });

    it('rejects a future day', () => {
        expect(TrackDayValidator.notInFuture(control(addDays(startOfToday(), 1)))).toEqual({
            future: true
        });
    });

    it('accepts an empty value', () => {
        expect(TrackDayValidator.notInFuture(control(null))).toBeNull();
    });
});
