import { FormControl } from '@angular/forms';
import { addDays, subDays } from 'date-fns';
import { describe, expect, it } from 'vitest';
import { FutureDateValidator } from './future-date.validator';

function control(value: Date | null): FormControl<Date | null> {
    return new FormControl<Date | null>(value, {
        validators: [FutureDateValidator.validate]
    });
}

describe('FutureDateValidator', () => {
    it('accepts a date in the future', () => {
        expect(control(addDays(new Date(), 1)).valid).toBe(true);
    });

    it('accepts today, whatever the time of day', () => {
        const today = new Date();
        today.setHours(0, 0, 1);
        expect(control(today).valid).toBe(true);
    });

    it('rejects a date in the past', () => {
        expect(control(subDays(new Date(), 1)).valid).toBe(false);
    });

    it('reports the failure as a past date error', () => {
        expect(control(subDays(new Date(), 1)).errors).toEqual({ pastDate: true });
    });

    it('stays quiet when no date is picked', () => {
        expect(control(null).valid).toBe(true);
    });
});
